package main

import (
	"fmt"
	"net/http"
	"strings"
)

func displayReport(results []Result) {
	fmt.Printf("\033[2K\rScanned: %d links\nErrors:  %d\nTime:    %vs\n\n", len(results), errorsProcessed, timeTaken)

	for _, r := range results {
		if r.StatusCode == 200 && len(r.Errors) == 0 && len(r.ValidationErrors) == 0 {
			continue
		}

		fmt.Println("---")
		fmt.Printf("Link:      %s\n", r.URL)
		fmt.Printf("Status:    %d (%s)\n", r.StatusCode, http.StatusText(r.StatusCode))
		if len(processed[r.URL]) > 0 {
			if len(processed[r.URL]) > 3 {
				fmt.Printf("Referrers: %s ... (%dx)\n", strings.Join(processed[r.URL][0:3], ", "), len(processed[r.URL]))
			} else {
				fmt.Printf("Referrers: %s\n", strings.Join(processed[r.URL], ", "))
			}
		}

		if len(r.Errors) > 0 || len(r.ValidationErrors) > 0 {
			fmt.Println("Errors:")
		}

		for _, e := range r.Errors {
			fmt.Printf(" - %s\n", e)
		}
		for _, e := range r.ValidationErrors {
			fmt.Printf(" [#%d] (%s) %s\n", e.LastLine, e.Type, strings.TrimSpace(e.Message))
		}
		fmt.Println("")
	}
}
