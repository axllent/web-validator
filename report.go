package main

import (
	"fmt"
	"net/http"
	"strings"
)

func displayReport(results []Result) {
	fmt.Printf("\033[2K\rScanned: %d links\nErrors:  %d\nTime:    %vs\n\n", linksProcessed, errorsProcessed, timeTaken)

	for _, r := range results {
		if r.StatusCode == 200 && len(r.Errors) == 0 && len(r.ValidationErrors) == 0 {
			continue
		}

		fmt.Printf("---\n\n")
		fmt.Printf("Link:    %s\n", r.URL)
		fmt.Printf("Status:  %d (%s)\n", r.StatusCode, http.StatusText(r.StatusCode))
		if len(referers[r.URL]) > 0 {
			if len(referers[r.URL]) > 3 {
				fmt.Printf("Refs:    %s ... (%dx)\n", strings.Join(referers[r.URL][0:3], "\n         "), len(referers[r.URL]))
			} else {
				fmt.Printf("Refs:    %s\n", strings.Join(referers[r.URL], "\n         "))
			}
		}

		if len(r.Errors) > 0 || len(r.ValidationErrors) > 0 {
			fmt.Println("Errors:")
		}

		errorNr := 0

		for _, e := range r.Errors {
			errorNr++
			fmt.Printf("  %4d)  [error] %s\n", errorNr, e)
		}
		for _, e := range r.ValidationErrors {
			errorNr++
			fmt.Printf("  %4d)  [#%d] (%s) %s\n", errorNr, e.LastLine, e.Type, strings.TrimSpace(e.Message))
		}
		fmt.Println("")
	}
}
