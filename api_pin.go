package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	ipfs "github.com/ipfs/go-ipfs-api"
)

// GxPinAPI for HTTP
type GxPinAPI struct {
	ipfs       *ipfs.Shell
	externalIP string
}

func newGxPinAPI(ipsh *ipfs.Shell, exip string) *GxPinAPI {
	return &GxPinAPI{
		ipfs:       ipsh,
		externalIP: exip,
	}
}

func (api *GxPinAPI) PinPackage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(403)
		return
	}

	ghurl := r.FormValue("ghurl")
	if !strings.HasPrefix(ghurl, "github.com/") {
		http.Error(w, "not a github url!", 400)
		return
	}

	userpkg := strings.Replace(ghurl, "github.com/", "", 1)

	template := "https://raw.githubusercontent.com/%s/master/.gx/lastpubver"
	url := fmt.Sprintf(template, userpkg)
	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer resp.Body.Close()

	out, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		http.Error(w, "incorrectly formatted lastpubver in repo", 400)
		return
	}
	vers := fields[0]
	hash := fields[1]

	flusher := w.(http.Flusher)

	fmt.Fprintln(w, "<!DOCTYPE html>")
	fmt.Fprintf(w, "<p>pinning %s version %s: %s</p><br>", ghurl, vers, hash)
	flusher.Flush()
	refs, err := api.ipfs.Refs(hash, true)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	fmt.Fprintln(w, "<ul>")
	for ref := range refs {
		fmt.Fprintf(w, "<li>%s</li>", ref)
		flusher.Flush()
	}
	fmt.Fprintln(w, "</ul>")

	fmt.Fprintln(w, "<p>fetched all deps!<br>calling pin now...</p>")
	flusher.Flush()

	err = api.ipfs.Pin(hash)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	err = logPin(ghurl, hash, vers)
	if err != nil {
		http.Error(w, fmt.Sprintf("writing log file: %s", err), 500)
		return
	}

	fmt.Fprintln(w, "<p>success!</p>")
	fmt.Fprintln(w, "<a href='/'>back</a>")

	recentlk.Lock()
	recent[ghurl] = pkgInfo{
		Url:     ghurl,
		Hash:    hash,
		Version: vers,
	}
	recentlk.Unlock()
}

func (api *GxPinAPI) Status(w http.ResponseWriter, r *http.Request) {
	if api.ipfs.IsUp() {
		fmt.Fprintf(w, "gexpin ipfs daemon is online!")
	} else {
		fmt.Fprintf(w, "gexpin ipfs daemon appears to be down. poke @whyrusleeping")
	}
}

func (api *GxPinAPI) NodeAddress(w http.ResponseWriter, r *http.Request) {
	myid, err := api.ipfs.ID()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}

	fmt.Fprintf(w, "/ip4/%s/tcp/4001/ipfs/%s", api.externalIP, myid.ID)
}

func (api *GxPinAPI) Recent(w http.ResponseWriter, r *http.Request) {
	recentlk.Lock()
	var pkgs []pkgInfo
	for _, p := range recent {
		pkgs = append(pkgs, p)
	}
	recentlk.Unlock()
	enc := json.NewEncoder(w)
	err := enc.Encode(pkgs)
	if err != nil {
		fmt.Println("json err: ", err)
		return
	}
}
