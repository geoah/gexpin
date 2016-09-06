package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"

	github "github.com/google/go-github/github"
	ipfs "github.com/ipfs/go-ipfs-api"
	gx "github.com/whyrusleeping/gx/gxutil"
	oauth2 "golang.org/x/oauth2"
)

type pkgInfo struct {
	Url     string
	Hash    string
	Version string
}

var recentlk sync.Mutex
var recent map[string]pkgInfo

var lk sync.Mutex

var log *os.File
var ghlogin *string

const pinlogFile = "pinlogs"

func init() {
	_, err := os.Stat(pinlogFile)
	if os.IsNotExist(err) {
		fi, err := os.Create(pinlogFile)
		if err != nil {
			panic(err)
		}

		log = fi
	} else {
		fi, err := os.OpenFile(pinlogFile, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			panic(err)
		}
		log = fi
	}

	recent = make(map[string]pkgInfo)
}

func main() {
	// parse flags
	listen := flag.String("listen", ":9444", "github token for stuff")
	ghtoken := flag.String("ghtoken", "", "github token for stuff")
	ghsecret := flag.String("ghsecret", "", "github token for stuff")
	ghlogin = flag.String("ghlogin", "geoah", "github bot login name") // TODO(geoah) Get this from the API
	flag.Parse()

	// find our external IP address
	myip, err := getExternalIP()
	if err != nil {
		fmt.Println("error getting external ip: ", err)
		os.Exit(1)
	}

	// new ipfs api
	ipsh := ipfs.NewShell("localhost:5001")

	// new gx pm
	cfg, err := gx.LoadConfig()
	if err != nil {
		fmt.Println("error loading gx config", err)
		os.Exit(1)
	}
	pm, err := gx.NewPM(cfg)
	if err != nil {
		fmt.Println("error getting new gx pm", err)
		os.Exit(1)
	}

	// new github client
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *ghtoken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	gh := github.NewClient(tc)

	// handle gx pin requests
	pinAPI := newGxPinAPI(ipsh, myip)
	http.HandleFunc("/pin_package", pinAPI.PinPackage)
	http.HandleFunc("/status", pinAPI.Status)
	http.HandleFunc("/node_addr", pinAPI.NodeAddress)
	http.HandleFunc("/recent", pinAPI.Recent)

	// handle github pr requets
	ghAPI := newGxGithubAPI(ipsh, gh, pm, myip, *ghsecret)
	http.HandleFunc("/github", ghAPI.Post)

	// serve static files
	// TODO(geoah) Move files into ./static or something.
	h := http.FileServer(http.Dir("."))
	http.Handle("/", h)

	// start http server
	http.ListenAndServe(*listen, nil)
}
