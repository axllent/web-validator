package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

var ignoreHosts = regexp.MustCompile(`^https?://(www\.)(linkedin\.com)`)

// HEAD a link to get the status of the URL
// Note: some sites block HEAD, so if a HEAD fails with a 404 or 405 error
// then a getResponse() is done (outbound links only)
func head(httplink string) {
	// check it's not a host that won't play ball, else ignore
	if ignoreHosts.MatchString(httplink) {
		return
	}

	output := Result{}
	output.URL = httplink
	timeout := time.Duration(10 * time.Second)

	client := http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequest("HEAD", httplink, nil)
	if err != nil {
		errorsProcessed++
		output.Errors = append(output.Errors, fmt.Sprintf("%s", err))
		results = append(results, output)
		return
	}

	req.Header.Set("User-Agent", userAgent)

	res, err := client.Do(req)
	if err != nil {
		output.Errors = append(output.Errors, fmt.Sprintf("%s", err))
		results = append(results, output)
		return
	}

	defer res.Body.Close()

	// some hosts block HEAD requests, so we do a standard GET instead
	if res.StatusCode == 404 || res.StatusCode == 405 {
		isOutbound := baseDomain != "" && getHost(httplink) != baseDomain

		if isOutbound {
			getResponse(httplink)
			return
		}
	}

	output.StatusCode = res.StatusCode

	if output.StatusCode != 200 {
		errorsProcessed++
	}

	results = append(results, output)
}

// Fallback for failed HEAD requests
func getResponse(httplink string) {
	// completely ignore requets to ignoreHosts
	if ignoreHosts.MatchString(httplink) {
		return
	}

	output := Result{}
	output.URL = httplink
	timeout := time.Duration(10 * time.Second)

	client := http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequest("GET", httplink, nil)
	if err != nil {
		errorsProcessed++
		output.Errors = append(output.Errors, fmt.Sprintf("%s", err))
		results = append(results, output)
		return
	}

	req.Header.Set("User-Agent", userAgent)

	res, err := client.Do(req)
	if err != nil {
		output.Errors = append(output.Errors, fmt.Sprintf("%s", err))
		results = append(results, output)
		return
	}

	defer res.Body.Close()

	output.StatusCode = res.StatusCode

	if output.StatusCode != 200 {
		errorsProcessed++
	}

	results = append(results, output)
}

// Return the domain name (host) from a URL
func getHost(httplink string) string {
	u, err := url.Parse(httplink)
	if err != nil {
		return ""
	}
	return u.Host
}

// AbsoluteURL will return a full URL regardless whether it is relative or absolute
func absoluteURL(link, baselink string, isHTML bool) (string, error) {
	u, err := url.Parse(link)
	if err != nil {
		return link, err
	}

	// remove hashes
	u.Fragment = ""

	base, err := url.Parse(baselink)
	if err != nil {
		return link, err
	}

	// set global variable
	if baseDomain == "" {
		baseDomain = base.Host
	}

	result := base.ResolveReference(u)

	// ensure link is HTTP(S)
	if result.Scheme != "http" && result.Scheme != "https" {
		return link, fmt.Errorf("Invalid URL: %s", result.String())
	}

	return result.String(), nil
}

// Whether related link is mixed content (HTTPS to HTTP).
func isMixedContent(srclink, reflink string) bool {
	srcLink, err := url.Parse(srclink)
	if err != nil || srcLink.Scheme == "http" {
		return false
	}

	refLink, err := url.Parse(reflink)
	if err == nil && refLink.Scheme == "http" {
		return true
	}

	return false
}

// Truncate a string
func truncateString(str string, num int) string {
	bnoden := str
	if len(str) > num {
		if num > 3 {
			num -= 3
		}
		bnoden = str[0:num] + "..."
	}
	return bnoden
}

// stringInSlice is a string-only version in php's in_array()
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// Single function to return a "weight" (int) based on the action to
// allow parsing of links that have already had a HEAD request (depth)
func actionWeight(f string) int {
	if f == "parse" {
		return 2
	}

	return 1
}

// Debugging pretty print
func prettyPrint(i interface{}) {
	s, _ := json.MarshalIndent(i, "", "\t")
	fmt.Println(string(s))
}
