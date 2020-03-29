package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/samclarke/robotstxt"
)

var (
	robots *robotstxt.RobotsTxt
)

// Set up robots.txt exclusions if allowed and exists
func initRobotsTxt(startURL string) {
	if noRobots {
		return
	}

	uri, err := url.Parse(startURL)
	if err != nil {
		noRobots = true
		return
	}

	robotsURL := fmt.Sprintf("%s://%s/robots.txt", uri.Scheme, uri.Host)

	timeout := time.Duration(time.Duration(timeoutSeconds) * time.Second)

	client := http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequest("GET", robotsURL, nil)
	if err != nil {
		noRobots = true
		return
	}

	req.Header.Set("User-Agent", userAgent)

	res, err := client.Do(req)
	if err != nil {
		noRobots = true
		return
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		noRobots = true
		return
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		noRobots = true
		return
	}

	robots, err = robotstxt.Parse(string(body), robotsURL)
	if err != nil {
		noRobots = true
		return
	}
}

// Test if allowed in robots.txt
func robotsAllowed(url string) bool {
	if noRobots {
		return true
	}

	if baseDomain != getHost(url) {
		return true
	}

	allowed, _ := robots.IsAllowed("web-validator", url)

	return allowed
}
