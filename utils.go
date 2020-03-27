package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	ignoreMatches = []*regexp.Regexp{
		regexp.MustCompile(`^https?://(www\.)?linkedin\.com`),
		regexp.MustCompile(`^https://(.*)\.google\.com`),
	}

	cssURLmatches = regexp.MustCompile(`(?mU)\burl\((.*)\)`)
)

// HEAD a link to get the status of the URL
// Note: some sites block HEAD, so if a HEAD fails with a 404 or 405 error
// then a getResponse() is done (outbound links only)
func head(httplink string, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()
	output := Result{}
	output.URL = httplink
	timeout := time.Duration(time.Duration(timeoutSeconds) * time.Second)

	client := http.Client{
		Timeout:       timeout,
		CheckRedirect: redirectMiddleware,
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
		errorsProcessed++
		if res != nil {
			loc := res.Header.Get("Location")
			output.StatusCode = res.StatusCode
			if loc != "" {
				full, err := absoluteURL(loc, httplink)
				if err == nil {
					output.Redirect = full
					results = append(results, output)
					addQueueLink(full, "head", httplink, 0, wg)
					return
				}
			}
		}
		output.Errors = append(output.Errors, fmt.Sprintf("%s", err))
		results = append(results, output)
		return
	}

	defer res.Body.Close()

	// some hosts block HEAD requests, so we do a standard GET instead
	if res.StatusCode == 404 || res.StatusCode == 405 {
		isOutbound := baseDomain != "" && getHost(httplink) != baseDomain

		if isOutbound {
			getResponse(httplink, wg)
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
func getResponse(httplink string, wg *sync.WaitGroup) {
	output := Result{}
	output.URL = httplink
	timeout := time.Duration(time.Duration(timeoutSeconds) * time.Second)

	client := http.Client{
		Timeout:       timeout,
		CheckRedirect: redirectMiddleware,
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
		errorsProcessed++
		if res != nil {
			loc := res.Header.Get("Location")
			output.StatusCode = res.StatusCode
			if loc != "" {
				full, err := absoluteURL(loc, httplink)
				if err == nil {
					output.Redirect = full
					results = append(results, output)
					addQueueLink(full, "head", httplink, 0, wg)
					return
				}
			}
		}
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
func absoluteURL(link, baselink string) (string, error) {
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
	if (result.Scheme != "http" && result.Scheme != "https") ||
		strings.HasPrefix(result.Path, "//") {
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

// RedirectMiddleware will return an error on redirect if redirectWarnings == true
func redirectMiddleware(req *http.Request, via []*http.Request) error {
	if redirectWarnings {
		return fmt.Errorf("%d redirect", req.Response.StatusCode)
	}
	return nil
}

// Debugging pretty print
func prettyPrint(i interface{}) {
	s, _ := json.MarshalIndent(i, "", "\t")
	fmt.Println(string(s))
}

// CSS URL
func extractStyleURLs(body string) []string {
	results := []string{}
	matches := cssURLmatches.FindAllStringSubmatch(body, -1)
	for _, res := range matches {
		url := strings.TrimSpace(res[1])
		// strip quotes left
		if len(url) > 0 && url[0] == '"' || url[0] == '\'' {
			url = url[1:]
		}
		// strip quotes right
		if len(url) > 0 && url[len(url)-1] == '"' || url[len(url)-1] == '\'' {
			url = url[:len(url)-1]
		}
		if len(url) > 0 {
			results = append(results, url)
		}
	}

	return results
}
