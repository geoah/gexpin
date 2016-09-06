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

		// deps, err := api.gx.EnumerateDependencies(pkg)
		// if err != nil {
		// 	fmt.Printf("> Could enumerate deps. err=%v\n", err)
		// 	return
		// }

		// for hash := range deps {
		// 	dep := pkg.FindDep(hash)
		// 	fmt.Println(">> Found dep", dep) // dep.Author, dep.Name, dep.Version, dep.Version)
		// }

		tree := make(map[string]*depTreeNode)
		deps := []*gx.Dependency{}

		var rec func(pkg *gx.Package) (*depTreeNode, error)
		rec = func(pkg *gx.Package) (*depTreeNode, error) {
			cur := new(depTreeNode)
			cur.this = new(gx.Dependency)
			err := pkg.ForEachDep(func(dep *gx.Dependency, dpkg *gx.Package) error {
				deps = append(deps, dep)
				sub := tree[dep.Hash]
				if sub == nil {
					var err error
					sub, err = rec(dpkg)
					if err != nil {
						return err
					}
					tree[dep.Hash] = sub
				}
				sub.this = dep
				cur.children = append(cur.children, sub)
				return nil
			})
			return cur, err
		}

		_, err = rec(pkg)
		if err != nil {
			fmt.Printf("> Could get all GX deps. err=%v\n", err)
			return
		}

		fmt.Printf("> Found %d dependecies.\n", len(deps))

		conflicts := make(map[string][]*gx.Dependency)

		for _, dep := range deps {
			// fmt.Println(">> Found dep", i, dep.Author, dep.Name, dep.Version, dep.Hash)
			for _, idep := range deps {
				if dep.Author == idep.Author && dep.Name == idep.Name && (dep.Version != idep.Version || dep.Hash != idep.Hash) {
					fmt.Printf(">> Found conflicting dependecies for %s v%s [%s] \n", dep.Name, dep.Version, dep.Hash)
					if _, ok := conflicts[dep.Name]; !ok {
						conflicts[dep.Name] = make([]*gx.Dependency, 0)
					}
					exists := false
					for _, cd := range conflicts[dep.Name] {
						if cd.Hash == dep.Hash {
							fmt.Println(">>> ", conflicts[dep.Name], cd.Hash, dep.Hash)
							exists = true
							break
						}
					}
					if exists == false {
						conflicts[dep.Name] = append(conflicts[dep.Name], dep)
					}
				}
			}
		}

		// fmt.Println(conflicts)

		report := ""
		if len(conflicts) > 0 {
			report = fmt.Sprintf("Found %d conflicting package/s.\n", len(conflicts))
			for _, conflict := range conflicts {
				if len(conflict) > 1 {
					report += fmt.Sprintf("* Package %s:\n", conflict[0].Name)
					for _, c := range conflict {
						report += fmt.Sprintf("  * %s [%s]\n", c.Version, c.Hash)
					}
				}
			}
		}

		fmt.Println(report)

		if report != "" {
			path := "package.json"
			pos := 1
			comment := &github.PullRequestComment{
				Body:     &report,
				CommitID: pr.Head.SHA,
				Path:     &path,
				Position: &pos,
			}
			_, _, err := api.github.PullRequests.CreateComment(*event.Repo.Owner.Login, *event.Repo.Name, *event.Number, comment)
			if err != nil {
				fmt.Printf("> Could comment on PR. err=%v\n", err)
				return
			}
		}
	}

}
