package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	github "github.com/google/go-github/github"
	ipfs "github.com/ipfs/go-ipfs-api"
)

// GxGithubAPI for Github webhooks
type GxGithubAPI struct {
	ipfs          *ipfs.Shell
	github        *github.Client
	externalIP    string
	webhookSecret string
}

func newGxGithubAPI(ipsh *ipfs.Shell, gh *github.Client, exip, whs string) *GxGithubAPI {
	return &GxGithubAPI{
		ipfs:          ipsh,
		github:        gh,
		externalIP:    exip,
		webhookSecret: whs,
	}
}

func (api *GxGithubAPI) Post(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, []byte(api.webhookSecret))
	if err != nil {
		fmt.Printf("Could not validate payload. err=%v;\n", err)
		return
	}

	fmt.Println(r.Header)
	fmt.Println(string(payload))

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

	fmt.Println(event)
}
