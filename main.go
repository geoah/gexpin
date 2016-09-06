package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"

	github "github.com/google/go-github/github"
	ipfs "github.com/ipfs/go-ipfs-api"
	"golang.org/x/oauth2"
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
	listen := flag.String("listen", ":9444", "github token for stuff")
	ghtoken := flag.String("ghtoken", "", "github token for stuff")
	ghsecret := flag.String("ghsecret", "", "github token for stuff")
	flag.Parse()

	myip, err := getExternalIP()
	if err != nil {
		fmt.Println("error getting external ip: ", err)
		os.Exit(1)
	}

	ipsh := ipfs.NewShell("localhost:5001")

	pinAPI := newGxPinAPI(ipsh, myip)
	http.HandleFunc("/pin_package", pinAPI.PinPackage)
	http.HandleFunc("/status", pinAPI.Status)
	http.HandleFunc("/node_addr", pinAPI.NodeAddress)
	http.HandleFunc("/recent", pinAPI.Recent)

	if *ghtoken != "" && *ghsecret != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: *ghtoken},
		)
		tc := oauth2.NewClient(oauth2.NoContext, ts)
		gh := github.NewClient(tc)

		ghAPI := newGxGithubAPI(ipsh, gh, myip, *ghsecret)
		http.HandleFunc("/github", ghAPI.Post)
	}

	h := http.FileServer(http.Dir("."))
	http.Handle("/", h)

	http.ListenAndServe(*listen, nil)
}
