package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	github "github.com/google/go-github/github"
	ipfs "github.com/ipfs/go-ipfs-api"
	gx "github.com/whyrusleeping/gx/gxutil"
)

// GxGithubAPI for Github webhooks
type GxGithubAPI struct {
	ipfs          *ipfs.Shell
	github        *github.Client
	gx            *gx.PM
	externalIP    string
	webhookSecret string
}

func newGxGithubAPI(ipsh *ipfs.Shell, gh *github.Client, gxPM *gx.PM, exip, whs string) *GxGithubAPI {
	return &GxGithubAPI{
		ipfs:          ipsh,
		github:        gh,
		gx:            gxPM,
		externalIP:    exip,
		webhookSecret: whs,
	}
}

// Post handles all github events
func (api *GxGithubAPI) Post(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, []byte(api.webhookSecret))
	if err != nil {
		fmt.Printf("Could not validate payload. err=%v;\n", err)
		return
	}

	// fmt.Println(r.Header)
	// fmt.Println(string(payload))

	et := r.Header.Get("X-Github-Event")
	if et != "pull_request" {
		fmt.Printf("We don't process anything other that PRs for now. et=%s;\n", et)
		return
	}

	event := &github.PullRequestEvent{}
	if err := json.Unmarshal(payload, event); err != nil {
		fmt.Printf("Could not unmarshal payload. err=%v;\n", err)
		return
	}

	// Some notes on the PR event.
	// Docs: https://developer.github.com/v3/activity/events/types/#pullrequestevent
	// Action:
	//   The action that was performed.
	//   Can be one of "assigned", "unassigned", "labeled", "unlabeled",
	//     "opened", "edited", "closed", or "reopened", or "synchronize".
	//   If the action is "closed" and the merged key is false, the pull
	//     request was closed with unmerged commits.
	//   If the action is "closed" and the merged key is true, the pull
	//     request was merged.

	// fmt.Println(event)

	// get the PR
	pr := event.PullRequest

	// TOOD(geoah) Extract to a provider or something.
	// if the PR is still open
	if *pr.State == "open" {
		// find an archived version of the HEAD
		url := fmt.Sprintf("https://github.com/%s/archive/%s.zip", *pr.Head.Repo.FullName, *pr.Head.SHA)

		rootDir := "./tmp"
		headZip := filepath.Join(rootDir, *pr.Head.SHA+".zip")
		headDir := filepath.Join(rootDir, *pr.Head.Repo.Name+"-"+*pr.Head.SHA)
		gxPkgFile := filepath.Join(headDir, "package.json")

		defer func() {
			// TODO(geoah) When all is said and done, clean up
			fmt.Printf("> Removing stuff around %s\n", headDir)
			// os.RemoveAll(headZip)
			// os.RemoveAll(headDir)
		}()

		// download it somewhere
		// TODO(geoah) Handle overwrites
		fmt.Printf("> Download HEAD from url. url=%s\n", url)
		err := download(url, headZip)
		if err != nil {
			fmt.Printf("> Could not download HEAD. err=%v\n", err)
			return
		}

		// TODO(geoah) Handle existing destination
		err = unzip(headZip, rootDir)
		if err != nil {
			fmt.Printf("> Could not unzip HEAD. err=%v\n", err)
			return
		}

		fmt.Printf("> We now have the unziped HEAD in %s\n", headDir)

		pkg, err := LoadPackageFile(gxPkgFile)
		if err != nil {
			fmt.Printf("> Could not read GX package. err=%v\n", err)
			return
		}

		deps, err := api.gx.EnumerateDependencies(pkg)
		if err != nil {
			fmt.Printf("> Could enumerate deps. err=%v\n", err)
			return
		}

		fmt.Printf("> Found %d dependecies. deps=%#v;\n", len(deps), deps)
	}

}
