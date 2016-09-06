package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func logPin(ghurl, ref, vers string) error {
	lk.Lock()
	defer lk.Unlock()
	_, err := fmt.Fprintf(log, "%s %s %s\n", ghurl, vers, ref)
	return err
}

func getExternalIP() (string, error) {
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	out, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(out), nil
}
