package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/css/scanner"
)

var (
	processed = make(map[string]int) // 1 = HEAD, 2 = GET
	referers  = make(map[string][]string)
	fileRegex = regexp.MustCompile(`(?i)\.(jpe?g|png|gif|svg|ico|pdf|docx?|xlsx?|zip|gz|bz2|tar|xz)$`)
	urlRegex  = regexp.MustCompile(`url\((.*)\)`)
)

// Link struct for channel
type Link struct {
	URL  string
	Type string
}

// Result struct
type Result struct {
	URL              string
	Type             string
	StatusCode       int
	Errors           []string
	ValidationErrors []ValidationError
}

// Add a link to the queue.
func addQueueLink(httplink, action, referer string, depth int) {
	if maxDepth != -1 && depth > maxDepth {
		// prevent further parsing by simply doing a HEAD
		action = "head"
	}

	// check if we have processed this already
	processType, found := processed[httplink]
	if found && processType >= actionWeight(action) {
		// add to referers
		if referer != httplink && !stringInSlice(referer, referers[httplink]) {
			referers[httplink] = append(referers[httplink], referer)
		}
	} else {
		// enforce HEAD - prevent validating common files HTML / CSS
		if action == "parse" && fileRegex.MatchString(httplink) {
			action = "head"
		}

		linksProcessed++
		processed[httplink] = actionWeight(action)

		// progress report
		fmt.Printf("\033[2K\r#%-3d (%d errors) %s", linksProcessed, errorsProcessed, truncateString(httplink, 100))

		if referer == "" {
			// initiate empty slice
			referers[httplink] = []string{}
		} else if referer != httplink {
			referers[httplink] = []string{referer}
		}

		isExternal := baseDomain != "" && getHost(httplink) != baseDomain

		if isExternal {
			if checkExternal {
				head(httplink)
			} else {
				linksProcessed--
				return
			}
		} else if action == "parse" {
			fetchAndParse(httplink, action, depth)
		} else {
			head(httplink)
		}
	}
}

// FetchAndParse will action a remove file, and
func fetchAndParse(httplink, action string, depth int) {
	output := Result{}
	output.URL = httplink
	output.Type = action

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
		errorsProcessed++
		output.Errors = append(output.Errors, fmt.Sprintf("%s", err))
		results = append(results, output)
		return
	}

	defer res.Body.Close()

	output.StatusCode = res.StatusCode

	if res.StatusCode != 200 {
		errorsProcessed++
		results = append(results, output)
		return
	}

	// read the body to create two separate readers
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		errorsProcessed++
		output.Errors = append(output.Errors, fmt.Sprintf("%s", err))
		results = append(results, output)
		return
	}

	// HTML
	if strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		// create separate *Reader for NuValidation
		r := bytes.NewReader(body)
		// validate the HTML
		output = validate(output, r, res.Header.Get("Content-Type"))

		// create a new reader
		r2 := bytes.NewReader(body)

		// Load the HTML document
		doc, err := goquery.NewDocumentFromReader(r2)
		if err != nil {
			output.Errors = append(output.Errors, fmt.Sprintf("%s", err))
			results = append(results, output)
			return
		}

		// CHECK FOR BASE
		baseLink := httplink

		doc.Find("base").Each(func(i int, s *goquery.Selection) {
			if link, ok := s.Attr("href"); ok {
				full, err := absoluteURL(link, httplink, true)
				if err == nil {
					baseLink = full
				}
			}
		})

		// IMAGES/VIDEOS/AUDIO/IFRAME
		doc.Find("img,embed,source,iframe").Each(func(i int, s *goquery.Selection) {
			if link, ok := s.Attr("src"); ok {
				full, err := absoluteURL(link, httplink, false)
				if err != nil {
					fmt.Println(err)
					return
				}
				if isMixedContent(httplink, full) {
					errorsProcessed++
					output.Errors = append(output.Errors, fmt.Sprintf("Mixed content to file: %s", full))
				}
				fileType := "head"
				// parse iframes as html
				if goquery.NodeName(s) == "iframe" {
					fileType = "parse"
				}
				addQueueLink(full, fileType, baseLink, depth)
			}
		})

		// CSS
		doc.Find("link[rel=\"stylesheet\"]").Each(func(i int, s *goquery.Selection) {
			if link, ok := s.Attr("href"); ok {
				full, err := absoluteURL(link, baseLink, false)
				if err != nil {
					fmt.Println(err)
					return
				}
				if isMixedContent(baseLink, full) {
					errorsProcessed++
					output.Errors = append(output.Errors, fmt.Sprintf("Mixed content link to CSS: %s", full))
				}
				addQueueLink(full, "parse", baseLink, depth)
			}
		})

		// JS
		doc.Find("script").Each(func(i int, s *goquery.Selection) {
			if link, ok := s.Attr("src"); ok {
				full, err := absoluteURL(link, baseLink, false)
				if err != nil {
					fmt.Println(err)
					return
				}
				if isMixedContent(baseLink, full) {
					errorsProcessed++
					output.Errors = append(output.Errors, fmt.Sprintf("Mixed content to JS: %s", full))
				}
				addQueueLink(full, "head", baseLink, depth)
			}
		})

		// FAVICONS
		doc.Find("link[rel=\"icon\"], link[rel=\"shortcut icon\"], link[rel=\"apple-touch-icon\"]").Each(func(i int, s *goquery.Selection) {
			if link, ok := s.Attr("href"); ok {
				full, err := absoluteURL(link, baseLink, false)
				if err != nil {
					fmt.Println(err)
					return
				}
				if isMixedContent(baseLink, full) {
					errorsProcessed++
					output.Errors = append(output.Errors, fmt.Sprintf("Mixed content to favicon: %s", full))
				}
				addQueueLink(full, "head", baseLink, depth)
			}
		})

		// LINKS
		doc.Find("a").Each(func(i int, s *goquery.Selection) {
			if link, ok := s.Attr("href"); ok {
				full, err := absoluteURL(link, baseLink, true)
				if err != nil {
					return
				}

				isExternal := baseDomain != "" && getHost(full) != baseDomain

				if isExternal {
					addQueueLink(full, "head", baseLink, depth)
				} else {
					addQueueLink(full, "parse", baseLink, depth+1)
				}
			}
		})
	}

	// CSS
	if strings.Contains(res.Header.Get("Content-Type"), "text/css") {
		// create separate *Reader for NuValidation
		r := bytes.NewReader(body)
		// validate the CSS
		output = validate(output, r, res.Header.Get("Content-Type"))

		s := scanner.New(string(body))

		for {
			token := s.Next()
			if token.Type == scanner.TokenEOF || token.Type == scanner.TokenError {
				break
			}

			if token.Type == scanner.TokenURI {
				matches := urlRegex.FindStringSubmatch(token.Value)
				if len(matches) > 0 {
					link := matches[1]
					// strip quotes off url() links
					link = strings.Replace(link, "'", "", -1)
					link = strings.Replace(link, "\"", "", -1)

					full, err := absoluteURL(link, httplink, false)
					if err != nil {
						// ignore failed asset links as they could be binary strings for svg etc
						continue
					}
					if isMixedContent(httplink, full) {
						errorsProcessed++
						output.Errors = append(output.Errors, fmt.Sprintf("Mixed content to CSS: %s", full))
					}
					addQueueLink(full, "head", httplink, depth)
				}
			}
		}
	}

	// append results to global
	results = append(results, output)
}
