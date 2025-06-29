// Package main is the application
package main

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/axllent/ghru/v2"
	"github.com/spf13/pflag"
)

var (
	results          []result
	maxDepth         int
	checkOutbound    bool
	validateHTML     bool
	validateCSS      bool
	showWarnings     bool
	baseDomain       string
	allLinks         bool
	fullScan         bool
	redirectWarnings bool
	noRobots         bool
	htmlValidator    = "https://validator.w3.org/nu/"
	timeTaken        float64
	update           bool
	showVersion      bool
	ignoreURLs       string
	timeoutSeconds   int
	threads          chan int
	appVersion       = "dev"
	userAgent        = "web-validator"
	linksProcessed   = 0
	errorsProcessed  = 0

	ghruConf = ghru.Config{
		Repo:           "axllent/web-validator",
		ArchiveName:    "web-validator-{{.OS}}-{{.Arch}}",
		BinaryName:     "web-validator",
		CurrentVersion: appVersion,
	}
)

func main() {

	showHelp := false
	var nrThreads int

	userAgent = fmt.Sprintf("web-validator/%s", appVersion)

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
	flag.StringVarP(&ignoreURLs, "ignore", "i", "", "ignore URLs, comma-separated, wildcards allowed (*.jpg,example.com)")
	flag.BoolVarP(&noRobots, "no-robots", "n", false, "ignore robots.txt (if exists)")
	flag.BoolVarP(&redirectWarnings, "redirects", "r", false, "treat redirects as errors")
	flag.BoolVarP(&showWarnings, "warnings", "w", false, "display validation warnings (default errors only)")
	flag.BoolVarP(&fullScan, "full", "f", false, "full scan (same as \"-a -r -o --html --css\")")
	flag.IntVarP(&nrThreads, "threads", "t", 5, "number of threads")
	flag.IntVar(&timeoutSeconds, "timeout", 10, "timeout in seconds")
	flag.StringVar(&htmlValidator, "validator", htmlValidator, "Nu Html validator")
	flag.BoolVarP(&update, "update", "u", false, "update to latest release")
	flag.BoolVarP(&showVersion, "version", "v", false, "show app version")
	flag.BoolVarP(&showHelp, "help", "h", false, "show help")

	_ = flag.MarkHidden("help")

	if err := flag.Parse(os.Args[1:]); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	args := flag.Args()

	if showHelp {
		fmt.Printf("Web-validator %s, validate website HTML & CSS, check links & resources.\n\n", appVersion)
		flag.Usage()
		fmt.Println("")
		os.Exit(0)
	}

	if showVersion {
		fmt.Printf("Version: %s\n", appVersion)

		release, err := ghruConf.Latest()
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		// The latest version is the same version
		if release.Tag == appVersion {
			os.Exit(0)
		}

		// A newer release is available
		fmt.Printf(
			"Update available: %s\nRun `%s -u` to update (requires read/write access to install directory).\n",
			release.Tag,
			os.Args[0],
		)
		os.Exit(0)
	}

	if update {
		// Update the app
		rel, err := ghruConf.SelfUpdate()
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		fmt.Printf("Updated %s to version %s\n", os.Args[0], rel.Tag)
		os.Exit(0)
	}

	if len(args) != 1 {
		fmt.Println("web-validator: missing URL")
		fmt.Printf("Try `%s -h` for more options.\n", os.Args[0])
		os.Exit(2)
	}

	if htmlValidator != "" {
		u, err := url.Parse(htmlValidator)
		if err != nil {
			fmt.Printf("Invalid Nu validator address: %s\n", htmlValidator)
			os.Exit(2)
		}

		q := u.Query()
		// add `?out=json`
		q.Set("out", "json")
		u.RawQuery = q.Encode()
		htmlValidator = u.String()
	}

	if ignoreURLs != "" {
		// create slice of ignore strings converting them to regex
		urls := strings.Split(ignoreURLs, ",")
		for _, u := range urls {
			filter := strings.ReplaceAll(u, "*", "WILDCARD_CHARACTER_HERE")
			filter = regexp.QuoteMeta(filter)
			filter = strings.ReplaceAll(filter, "WILDCARD_CHARACTER_HERE", "(.*)")
			re := regexp.MustCompile(filter)
			ignoreMatches = append(ignoreMatches, re)
		}
	}

	if fullScan {
		maxDepth = -1
		checkOutbound = true
		validateHTML = true
		validateCSS = true
		redirectWarnings = true
	}

	if allLinks {
		maxDepth = -1
	}

	u, err := url.Parse(args[0])
	if err != nil || u.Host == "" {
		fmt.Printf("Please use a full URL: %s\n", args[0])
		os.Exit(2)
	}

	initRobotsTxt(args[0])

	threads = make(chan int, nrThreads)

	start := time.Now()

	var wg sync.WaitGroup

	addQueueLink(args[0], "parse", "", 0, &wg)

	// display results if process is cancelled (ctrl-c)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			elapsed := time.Since(start)

			timeTaken = elapsed.Round(time.Second).Seconds()
			fmt.Println("")
			fmt.Println("Process interrupted")
			displayReport(results)
			os.Exit(1)
		}
	}()

	wg.Wait()

	elapsed := time.Since(start)

	timeTaken = elapsed.Round(time.Second).Seconds()

	displayReport(results)
}
