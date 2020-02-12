package main

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/axllent/ghru"
	"github.com/spf13/pflag"
)

var (
	results         []Result
	uri             string
	maxDepth        int
	checkOutbound   bool
	validateHTML    bool
	validateCSS     bool
	showWarnigs     bool
	baseDomain      string
	allLinks        bool
	fullScan        bool
	htmlValidator   = "https://validator.w3.org/nu/"
	timeTaken       float64
	update          bool
	showVersion     bool
	appVersion      = "dev"
	userAgent       = "Web-validator"
	linksProcessed  = 0
	errorsProcessed = 0
)

func main() {

	showHelp := false

	userAgent = fmt.Sprintf("Web-validator/%s", appVersion)

	flag := pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)

	// set the default help
	flag.Usage = func() {
		fmt.Printf("Usage: %s [options] <url>\n\n", os.Args[0])
		fmt.Println("Options:")
		flag.SortFlags = false
		flag.PrintDefaults()
	}

	flag.BoolVarP(&allLinks, "all", "a", false, "recursive, follow all internal links (default single URL)")
	flag.IntVarP(&maxDepth, "depth", "d", 0, "crawl depth (\"-a\" will override this)")
	flag.BoolVarP(&checkOutbound, "outbound", "o", false, "check outbound links (HEAD only)")
	flag.BoolVar(&validateHTML, "html", false, "validate HTML")
	flag.BoolVar(&validateCSS, "css", false, "validate CSS")
	flag.BoolVarP(&showWarnigs, "warnings", "w", false, "display warnings too (default only show errors)")
	flag.BoolVarP(&fullScan, "full", "f", false, "full scan (same as \"-a -o --html --css\")")
	flag.StringVar(&htmlValidator, "validator", htmlValidator, "Nu Html validator")
	flag.BoolVarP(&update, "update", "u", false, "update to latest release")
	flag.BoolVarP(&showVersion, "version", "v", false, "show app version")
	flag.BoolVarP(&showHelp, "help", "h", false, "show help")

	flag.MarkHidden("help")

	flag.Parse(os.Args[1:])

	args := flag.Args()

	if showHelp {
		fmt.Printf("Web-validator %s, validate website HTML & CSS, check links & resources.\n\n", appVersion)
		flag.Usage()
		os.Exit(0)
	}

	if showVersion {
		fmt.Println(fmt.Sprintf("Version: %s", appVersion))
		latest, _, _, err := ghru.Latest("axllent/web-validator", "web-validator")
		if err == nil && ghru.GreaterThan(latest, appVersion) {
			fmt.Printf("Update available: %s\nRun `%s -u` to update.\n", latest, os.Args[0])
		}
		os.Exit(0)
	}

	if update {
		rel, err := ghru.Update("axllent/web-validator", "web-validator", appVersion)
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		fmt.Printf("Updated %s to version %s\n", os.Args[0], rel)
		os.Exit(0)
	}

	if len(args) != 1 {
		fmt.Println("web-validator: missing URL")
		fmt.Printf("Try `%s -h` for more options.\n", os.Args[0])
		os.Exit(2)
	}

	start := time.Now()

	if htmlValidator != "" {
		u, err := url.Parse(htmlValidator)
		if err != nil {
			fmt.Printf("Invalid Nu validator address: %s\n", htmlValidator)
			os.Exit(2)
		}
		// add `?out=json`
		q := u.Query()
		q.Set("out", "json")
		u.RawQuery = q.Encode()
		htmlValidator = u.String()
	}

	if fullScan {
		maxDepth = -1
		checkOutbound = true
		validateHTML = true
		validateCSS = true
	}

	if allLinks {
		maxDepth = -1
	}

	addQueueLink(args[0], "parse", "", 0)

	elapsed := time.Since(start)

	timeTaken = elapsed.Round(time.Second).Seconds()

	displayReport(results)
}
