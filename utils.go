package main

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	crawlMutex        sync.Mutex
	lastCrawlTime     time.Time
	validatorMux      sync.Mutex
	lastValidatorTime time.Time

	ignoreMatches = []*regexp.Regexp{}

	cssURLmatches = regexp.MustCompile(`(?mU)\burl\((.*)\)`)
)

// applyAuth sets HTTP basic auth on the request when credentials are configured
// and the request targets the primary site. Outbound requests are left untouched.
func applyAuth(req *http.Request) {
	if authUser == "" {
		return
	}
	if baseDomain == "" || req.URL.Host != baseDomain {
		return
	}
	req.SetBasicAuth(authUser, authPassword)
}

// crawlWait enforces the crawl delay by serialising requests: each caller
// waits until crawlDelay has elapsed since the last request was made.
func crawlWait() {
	if crawlDelay == 0 {
		return
	}
	crawlMutex.Lock()
	defer crawlMutex.Unlock()
	if !lastCrawlTime.IsZero() {
		if elapsed := time.Since(lastCrawlTime); elapsed < crawlDelay {
			time.Sleep(crawlDelay - elapsed)
		}
	}
	lastCrawlTime = time.Now()
}

// validatorWait enforces the validator delay using the same pattern as crawlWait.
func validatorWait() {
	if validatorDelay == 0 {
		return
	}
	validatorMux.Lock()
	defer validatorMux.Unlock()
	if !lastValidatorTime.IsZero() {
		if elapsed := time.Since(lastValidatorTime); elapsed < validatorDelay {
			time.Sleep(validatorDelay - elapsed)
		}
	}
	lastValidatorTime = time.Now()
}

// HEAD a link to get the status of the URL
// Note: some sites block HEAD, so if a HEAD fails with a 404 or 405 error
// then a getResponse() is performed is done (outbound links only)
func head(httpLink string, wg *sync.WaitGroup) {
	defer wg.Done()
	output := result{}
	output.URL = httpLink
	timeout := time.Duration(time.Duration(timeoutSeconds) * time.Second)

	client := http.Client{
		Timeout:       timeout,
		CheckRedirect: redirectMiddleware,
	}

	req, err := http.NewRequest("HEAD", httpLink, nil)
	if err != nil {
		errorsProcessed.Add(1)
		output.Errors = append(output.Errors, fmt.Sprintf("%s", err))
		appendResult(output)
		return
	}

	req.Header.Set("User-Agent", userAgent)
	applyAuth(req)

	res, err := client.Do(req)
	if err != nil {
		errorsProcessed.Add(1)
		if res != nil {
			loc := res.Header.Get("Location")
			output.StatusCode = res.StatusCode
			if loc != "" {
				full, err := absoluteURL(loc, httpLink)
				if err == nil {
					output.Redirect = full
					appendResult(output)
					addQueueLink(full, "head", httpLink, 0, wg)
					return
				}
			}
		}
		output.Errors = append(output.Errors, fmt.Sprintf("%s", err))
		appendResult(output)
		return
	}

	defer func() { _ = res.Body.Close() }()

	// some hosts block HEAD requests, so we do a standard GET instead
	if res.StatusCode == 404 || res.StatusCode == 405 {
		isOutbound := baseDomain != "" && getHost(httpLink) != baseDomain

		if isOutbound {
			getResponse(httpLink, wg)
			return
		}
	}

	output.StatusCode = res.StatusCode

	if output.StatusCode != 200 {
		errorsProcessed.Add(1)
		output.Errors = append(output.Errors, fmt.Sprintf("returned status %d", output.StatusCode))
	}

	appendResult(output)
}

// Fallback for failed HEAD requests
func getResponse(httpLink string, wg *sync.WaitGroup) {
	crawlWait()
	output := result{}
	output.URL = httpLink
	timeout := time.Duration(time.Duration(timeoutSeconds) * time.Second)

	client := http.Client{
		Timeout:       timeout,
		CheckRedirect: redirectMiddleware,
	}

	req, err := http.NewRequest("GET", httpLink, nil)
	if err != nil {
		errorsProcessed.Add(1)
		output.Errors = append(output.Errors, fmt.Sprintf("%s", err))
		appendResult(output)
		return
	}

	req.Header.Set("User-Agent", userAgent)
	applyAuth(req)

	res, err := client.Do(req)
	if err != nil {
		errorsProcessed.Add(1)
		if res != nil {
			loc := res.Header.Get("Location")
			output.StatusCode = res.StatusCode
			if loc != "" {
				full, err := absoluteURL(loc, httpLink)
				if err == nil {
					output.Redirect = full
					appendResult(output)
					addQueueLink(full, "head", httpLink, 0, wg)
					return
				}
			}
		}
		output.Errors = append(output.Errors, fmt.Sprintf("%s", err))
		appendResult(output)
		return
	}

	defer func() { _ = res.Body.Close() }()

	output.StatusCode = res.StatusCode

	if output.StatusCode != 200 {
		errorsProcessed.Add(1)
		output.Errors = append(output.Errors, fmt.Sprintf("returned status %d", output.StatusCode))
	}

	appendResult(output)
}

// Return the domain name (host) from a URL
func getHost(httpLink string) string {
	u, err := url.Parse(httpLink)
	if err != nil {
		return ""
	}
	return u.Host
}

// AbsoluteURL will return a full URL regardless whether it is relative or absolute
func absoluteURL(link, baseLink string) (string, error) {
	// scheme relative links, eg <script src="//example.com/script.js">
	if len(link) > 1 && link[0:2] == "//" {
		base, err := url.Parse(baseLink)
		if err != nil {
			return link, err
		}
		link = base.Scheme + ":" + link
	}

	u, err := url.Parse(link)
	if err != nil {
		return link, err
	}

	// remove hashes
	u.Fragment = ""

	base, err := url.Parse(baseLink)
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
		return link, fmt.Errorf("invalid URL: %s", result.String())
	}

	return result.String(), nil
}

// Whether related link is mixed content (HTTPS to HTTP).
func isMixedContent(src, ref string) bool {
	srcLink, err := url.Parse(src)
	if err != nil || srcLink.Scheme == "http" {
		return false
	}

	refLink, err := url.Parse(ref)
	if err == nil && refLink.Scheme == "http" {
		return true
	}

	return false
}

// Truncate a string
func truncateString(str string, num int) string {
	ts := str
	if len(str) > num {
		if num > 3 {
			num -= 3
		}
		ts = str[0:num] + "..."
	}
	return ts
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
func redirectMiddleware(req *http.Request, _ []*http.Request) error {
	if redirectWarnings {
		return fmt.Errorf("%d redirect", req.Response.StatusCode)
	}
	return nil
}

// CSS URL
func extractStyleURLs(body string) []string {
	results := []string{}
	matches := cssURLmatches.FindAllStringSubmatch(body, -1)
	for _, res := range matches {
		url := strings.TrimSpace(res[1])
		// strip quotes left
		if len(url) > 0 && (url[0] == '"' || url[0] == '\'') {
			url = url[1:]
		}
		// strip quotes right
		if len(url) > 0 && (url[len(url)-1] == '"' || url[len(url)-1] == '\'') {
			url = url[:len(url)-1]
		}
		if len(url) > 0 {
			results = append(results, url)
		}
	}

	return results
}
