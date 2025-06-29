package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/lukasbob/srcset"
	"golang.org/x/exp/slices"
)

var (
	processed      = make(map[string]int) // 1 = HEAD, 2 = GET
	referrers      = make(map[string][]string)
	mapMutex       = sync.RWMutex{}
	validatorMutex = sync.RWMutex{}
	fileRegex      = regexp.MustCompile(`(?i)\.(jpe?g|png|gif|svg|ico|pdf|swf|mp4|avi|mp3|ogg|mkv|docx?|xlsx?|zip|gz|bz2|tar|xz)$`)
)

// Result struct
type result struct {
	URL              string
	Type             string
	StatusCode       int
	Errors           []string
	ValidationErrors []validationError
	Redirect         string
}

// Add a link to the queue.
func addQueueLink(httpLink, action, referer string, depth int, wg *sync.WaitGroup) {
	if !robotsAllowed(httpLink) {
		return
	}

	if maxDepth != -1 && depth > maxDepth {
		// prevent further parsing by simply doing a HEAD
		action = "head"
	}

	// remove trailing ? or #
	if len(httpLink) > 0 && httpLink[len(httpLink)-1] == '?' || httpLink[len(httpLink)-1] == '#' {
		httpLink = httpLink[:len(httpLink)-1]
	}

	for _, r := range ignoreMatches {
		if r.MatchString(httpLink) {
			return
		}
	}

	isOutbound := baseDomain != "" && getHost(httpLink) != baseDomain

	if isOutbound && !checkOutbound {
		return
	}

	threads <- 1 // will block if there is MAX ints in threads

	wg.Add(1)
	defer wg.Done()

	// ensure only one process can read/write to processed map
	mapMutex.Lock()
	defer mapMutex.Unlock()

	// check if we have processed this already
	processType, found := processed[httpLink]
	if found && processType >= actionWeight(action) {
		// add to referrers
		if referer != httpLink && !slices.Contains(referrers[httpLink], referer) {
			referrers[httpLink] = append(referrers[httpLink], referer)
		}
	} else {
		// enforce HEAD - prevent validating common files HTML / CSS
		if action == "parse" && fileRegex.MatchString(httpLink) {
			action = "head"
		}

		linksProcessed++
		processed[httpLink] = actionWeight(action)

		// progress report
		fmt.Printf("\033[2K\r#%-3d (%d errors) %s", linksProcessed, errorsProcessed, truncateString(httpLink, 100))

		if referer == "" {
			// initiate empty slice
			referrers[httpLink] = []string{}
		} else if referer != httpLink {
			referrers[httpLink] = []string{referer}
		}

		if isOutbound {
			go head(httpLink, wg)
		} else if action == "parse" {
			go fetchAndParse(httpLink, action, depth, wg)
		} else {
			go head(httpLink, wg)
		}
		// add small delay to ensure goroutine registers wg.Add(1) before completion
		time.Sleep(time.Millisecond * 100)
	}

	<-threads // removes an int from threads, allowing another to proceed
}

// FetchAndParse will request the URL and parse it.
func fetchAndParse(httpLink, action string, depth int, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()
	output := result{}
	output.URL = httpLink
	output.Type = action

	timeout := time.Duration(time.Duration(timeoutSeconds) * time.Second)

	client := http.Client{
		Timeout:       timeout,
		CheckRedirect: redirectMiddleware,
	}

	req, err := http.NewRequest("GET", httpLink, nil)
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
				full, err := absoluteURL(loc, httpLink)
				if err == nil {
					output.Redirect = full
					results = append(results, output)
					addQueueLink(full, action, httpLink, depth, wg)
					return
				}
			}
		}
		output.Errors = append(output.Errors, fmt.Sprintf("%s", err))
		results = append(results, output)
		return
	}

	defer func() { _ = res.Body.Close() }()

	output.StatusCode = res.StatusCode

	if res.StatusCode != 200 {
		errorsProcessed++
		results = append(results, output)
		return
	}

	// read the body to create two separate readers
	body, err := io.ReadAll(res.Body)
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
		baseLink := httpLink

		doc.Find("base").Each(func(i int, s *goquery.Selection) {
			if link, ok := s.Attr("href"); ok {
				full, err := absoluteURL(link, httpLink)
				if err == nil {
					baseLink = full
				}
			}
		})

		// IMAGES/VIDEOS/AUDIO/IFRAME
		doc.Find("img,embed,source,iframe").Each(func(i int, s *goquery.Selection) {
			if link, ok := s.Attr("src"); ok {
				full, err := absoluteURL(link, baseLink)
				if err != nil {
					return
				}
				if isMixedContent(httpLink, full) {
					errorsProcessed++
					output.Errors = append(output.Errors, fmt.Sprintf("Mixed content to file: %s", full))
				}
				fileType := "head"
				// parse iframes as html
				if goquery.NodeName(s) == "iframe" {
					fileType = "parse"
				}
				addQueueLink(full, fileType, httpLink, depth, wg)
			}

			if link, ok := s.Attr("srcset"); ok {
				// srcset may contain multiple urls
				srcLinks := srcset.Parse(link)
				for _, src := range srcLinks {
					link := src.URL
					full, err := absoluteURL(link, baseLink)
					if err != nil {
						return
					}
					if isMixedContent(httpLink, full) {
						errorsProcessed++
						output.Errors = append(output.Errors, fmt.Sprintf("Mixed content to file: %s", full))
					}
					addQueueLink(full, "head", httpLink, depth, wg)
				}
			}
		})

		// CSS
		doc.Find("link[rel=\"stylesheet\"]").Each(func(i int, s *goquery.Selection) {
			if link, ok := s.Attr("href"); ok {
				full, err := absoluteURL(link, baseLink)
				if err != nil {
					fmt.Println(err)
					return
				}
				if isMixedContent(baseLink, full) {
					errorsProcessed++
					output.Errors = append(output.Errors, fmt.Sprintf("Mixed content link to CSS: %s", full))
				}
				addQueueLink(full, "parse", httpLink, depth, wg)
			}
		})

		// JS
		doc.Find("script").Each(func(i int, s *goquery.Selection) {
			if link, ok := s.Attr("src"); ok {
				full, err := absoluteURL(link, baseLink)
				if err != nil {
					fmt.Println(err)
					return
				}
				if isMixedContent(baseLink, full) {
					errorsProcessed++
					output.Errors = append(output.Errors, fmt.Sprintf("Mixed content to JS: %s", full))
				}
				addQueueLink(full, "head", httpLink, depth, wg)
			}
		})

		// FAVICONS
		doc.Find("link[rel=\"icon\"], link[rel=\"shortcut icon\"], link[rel=\"apple-touch-icon\"]").Each(func(i int, s *goquery.Selection) {
			if link, ok := s.Attr("href"); ok {
				full, err := absoluteURL(link, baseLink)
				if err != nil {
					return
				}
				if isMixedContent(baseLink, full) {
					errorsProcessed++
					output.Errors = append(output.Errors, fmt.Sprintf("Mixed content to favicon: %s", full))
				}
				addQueueLink(full, "head", httpLink, depth, wg)
			}
		})

		// OPEN GRAPH IMAGES
		doc.Find("meta[property$=\":image\"], meta[name$=\":image\"]").Each(func(i int, s *goquery.Selection) {
			if link, ok := s.Attr("content"); ok {
				full, err := absoluteURL(link, baseLink)
				if err != nil {
					return
				}
				if isMixedContent(baseLink, full) {
					errorsProcessed++
					output.Errors = append(output.Errors, fmt.Sprintf("Mixed content to favicon: %s", full))
				}
				addQueueLink(full, "head", httpLink, depth, wg)
			}
		})

		// LINKS
		doc.Find("a").Each(func(i int, s *goquery.Selection) {
			if link, ok := s.Attr("href"); ok {
				full, err := absoluteURL(link, baseLink)
				if err != nil {
					return
				}

				isOutbound := baseDomain != "" && getHost(full) != baseDomain

				if isOutbound {
					addQueueLink(full, "head", httpLink, depth, wg)
				} else {
					addQueueLink(full, "parse", httpLink, depth+1, wg)
				}
			}
		})

		// INLINE STYLE BLOCKS
		doc.Find("style").Each(func(i int, s *goquery.Selection) {
			raw := s.Text()
			for _, link := range extractStyleURLs(raw) {
				full, err := absoluteURL(link, baseLink)
				if err != nil {
					break
				}
				if isMixedContent(httpLink, full) {
					errorsProcessed++
					output.Errors = append(output.Errors, fmt.Sprintf("Mixed content from CSS: %s", full))
				}
				addQueueLink(full, "head", httpLink, depth, wg)
			}
		})

		// INLINE STYLES
		doc.Find("*[style]").Each(func(i int, s *goquery.Selection) {
			if style, ok := s.Attr("style"); ok {
				for _, link := range extractStyleURLs(style) {
					full, err := absoluteURL(link, baseLink)
					if err != nil {
						return
					}
					if isMixedContent(httpLink, full) {
						errorsProcessed++
						output.Errors = append(output.Errors, fmt.Sprintf("Mixed content from CSS: %s", full))
					}
					addQueueLink(full, "head", httpLink, depth, wg)
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

		for _, link := range extractStyleURLs(string(body)) {
			full, err := absoluteURL(link, httpLink)
			if err != nil {
				continue
			}
			if isMixedContent(httpLink, full) {
				errorsProcessed++
				output.Errors = append(output.Errors, fmt.Sprintf("Mixed content from CSS: %s", full))
			}
			addQueueLink(full, "head", httpLink, depth, wg)
		}
	}

	// append results to global
	results = append(results, output)
}
