package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// sitemapDoc handles both <urlset> and <sitemapindex> documents.
type sitemapDoc struct {
	XMLName  xml.Name `xml:""`
	URLs     []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

// seedFromSitemap fetches /sitemap.xml from the base URL and adds discovered
// URLs to the processing queue. If the sitemap does not exist or cannot be
// parsed, it returns silently so the normal scan continues unaffected.
func seedFromSitemap(startURL string, wg *sync.WaitGroup) {
	uri, err := url.Parse(startURL)
	if err != nil {
		return
	}

	sitemapURL := fmt.Sprintf("%s://%s/sitemap.xml", uri.Scheme, uri.Host)
	fetchSitemap(sitemapURL, wg)
}

// fetchSitemap fetches and parses a sitemap URL, queuing all discovered page
// URLs. Sitemap index documents are followed recursively.
func fetchSitemap(sitemapURL string, wg *sync.WaitGroup) {
	timeout := time.Duration(time.Duration(timeoutSeconds) * time.Second)

	client := http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequest("GET", sitemapURL, nil)
	if err != nil {
		return
	}

	req.Header.Set("User-Agent", userAgent)

	res, err := client.Do(req)
	if err != nil {
		return
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != 200 {
		return
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return
	}

	var doc sitemapDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return
	}

	// Sitemap index: recurse into each child sitemap.
	if doc.XMLName.Local == "sitemapindex" {
		for _, s := range doc.Sitemaps {
			if s.Loc != "" {
				fetchSitemap(s.Loc, wg)
			}
		}
		return
	}

	// URL set: queue each listed URL for parsing.
	for _, u := range doc.URLs {
		if u.Loc != "" {
			addQueueLink(u.Loc, "parse", sitemapURL, 0, wg)
		}
	}
}
