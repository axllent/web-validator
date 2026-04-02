package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func displayReport(results []result) {
	if outputFormat != "text" {
		fmt.Fprintln(os.Stderr)
	}
	switch outputFormat {
	case "json":
		reportJSON(results)
	case "csv":
		reportCSV(results)
	case "html":
		reportHTML(results)
	default:
		reportText(results)
	}
}

func reportText(results []result) {
	fmt.Printf("\033[2K\rScanned: %d links\nErrors:  %d\nTime:    %vs\n\n", linksProcessed, errorsProcessed.Load(), timeTaken)

	for _, r := range results {
		if r.StatusCode == 200 && len(r.Errors) == 0 && len(r.ValidationErrors) == 0 && r.Redirect == "" {
			continue
		}

		fmt.Printf("---\n\n")

		if r.Redirect != "" {
			fmt.Printf("Link:    %s => %s\n", r.URL, r.Redirect)
		} else {
			fmt.Printf("Link:    %s\n", r.URL)
		}

		if r.StatusCode > 0 {
			fmt.Printf("Status:  %d (%s)\n", r.StatusCode, http.StatusText(r.StatusCode))
		}

		if len(referrers[r.URL]) > 0 {
			if len(referrers[r.URL]) > 3 {
				fmt.Printf("Refs:    %s ... (%dx)\n", strings.Join(referrers[r.URL][0:3], "\n         "), len(referrers[r.URL]))
			} else {
				fmt.Printf("Refs:    %s\n", strings.Join(referrers[r.URL], "\n         "))
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

func reportJSON(results []result) {
	type issue struct {
		URL              string            `json:"url"`
		Status           int               `json:"status,omitempty"`
		Redirect         string            `json:"redirect,omitempty"`
		Referrers        []string          `json:"referrers"`
		Errors           []string          `json:"errors"`
		ValidationErrors []validationError `json:"validation_errors"`
	}
	type report struct {
		Scanned int     `json:"scanned"`
		Errors  int64   `json:"errors"`
		Time    float64 `json:"time"`
		Issues  []issue `json:"issues"`
	}

	rep := report{
		Scanned: linksProcessed,
		Errors:  errorsProcessed.Load(),
		Time:    timeTaken,
		Issues:  []issue{},
	}

	for _, r := range results {
		if r.StatusCode == 200 && len(r.Errors) == 0 && len(r.ValidationErrors) == 0 && r.Redirect == "" {
			continue
		}

		refs := referrers[r.URL]
		if refs == nil {
			refs = []string{}
		}
		errs := r.Errors
		if errs == nil {
			errs = []string{}
		}
		valErrs := r.ValidationErrors
		if valErrs == nil {
			valErrs = []validationError{}
		}

		rep.Issues = append(rep.Issues, issue{
			URL:              r.URL,
			Status:           r.StatusCode,
			Redirect:         r.Redirect,
			Referrers:        refs,
			Errors:           errs,
			ValidationErrors: valErrs,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rep)
}

func reportHTML(results []result) {
	type issueRow struct {
		URL              string
		Status           int
		StatusText       string
		Redirect         string
		Referrers        []string
		Errors           []string
		ValidationErrors []validationError
	}

	type data struct {
		BaseURL   string
		Scanned   int
		Errors    int64
		Time      float64
		HasIssues bool
		Issues    []issueRow
	}

	d := data{
		BaseURL: baseDomain,
		Scanned: linksProcessed,
		Errors:  errorsProcessed.Load(),
		Time:    timeTaken,
	}

	for _, r := range results {
		if r.StatusCode == 200 && len(r.Errors) == 0 && len(r.ValidationErrors) == 0 && r.Redirect == "" {
			continue
		}
		d.Issues = append(d.Issues, issueRow{
			URL:              r.URL,
			Status:           r.StatusCode,
			StatusText:       http.StatusText(r.StatusCode),
			Redirect:         r.Redirect,
			Referrers:        referrers[r.URL],
			Errors:           r.Errors,
			ValidationErrors: r.ValidationErrors,
		})
	}
	d.HasIssues = len(d.Issues) > 0

	const tmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>web-validator report &mdash; {{.BaseURL}}</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:system-ui,sans-serif;font-size:14px;color:#1a1a1a;background:#f5f5f5;padding:24px}
h1{font-size:20px;font-weight:600;margin-bottom:4px}
.meta{color:#555;margin-bottom:24px;font-size:13px}
.meta span{margin-right:16px}
.ok{color:#16a34a;font-weight:600}
.none{color:#555;font-style:italic}
table{width:100%;border-collapse:collapse;background:#fff;border-radius:6px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,.1)}
thead{background:#1a1a1a;color:#fff}
th,td{padding:10px 14px;text-align:left;vertical-align:top}
th{font-weight:500;font-size:13px;white-space:nowrap}
tr:not(:last-child) td{border-bottom:1px solid #e5e5e5}
tr:hover td{background:#fafafa}
.url{word-break:break-all;font-family:monospace;font-size:13px}
.badge{display:inline-block;padding:2px 7px;border-radius:4px;font-size:12px;font-weight:600}
.badge-err{background:#fee2e2;color:#b91c1c}
.badge-red{background:#fee2e2;color:#b91c1c}
.badge-ok{background:#dcfce7;color:#15803d}
.badge-redir{background:#fef9c3;color:#854d0e}
ul{list-style:none;padding:0}
ul li{padding:2px 0;color:#374151}
ul li.verr{color:#7c3aed}
ul li.ref{font-family:monospace;font-size:12px;color:#555;word-break:break-all}
</style>
</head>
<body>
<h1>web-validator &mdash; {{.BaseURL}}</h1>
<p class="meta">
  <span>Scanned: <strong>{{.Scanned}}</strong> links</span>
  <span>Errors: <strong>{{.Errors}}</strong></span>
  <span>Time: <strong>{{.Time}}s</strong></span>
</p>
{{if .HasIssues}}
<table>
<thead><tr><th>URL</th><th>Status</th><th>Referrers</th><th>Errors</th></tr></thead>
<tbody>
{{range .Issues}}
<tr>
  <td class="url">
    {{if .Redirect}}
      <a href="{{.URL}}" target="_blank">{{.URL}}</a>
      &rarr; <a href="{{.Redirect}}" target="_blank">{{.Redirect}}</a>
    {{else}}
      <a href="{{.URL}}" target="_blank">{{.URL}}</a>
    {{end}}
  </td>
  <td>
    {{if .Status}}
      {{if .Redirect}}
        <span class="badge badge-redir">{{.Status}} {{.StatusText}}</span>
      {{else if eq .Status 200}}
        <span class="badge badge-ok">{{.Status}}</span>
      {{else}}
        <span class="badge badge-red">{{.Status}} {{.StatusText}}</span>
      {{end}}
    {{end}}
  </td>
  <td>
    <ul>
      {{range .Referrers}}<li class="ref">{{.}}</li>{{end}}
    </ul>
  </td>
  <td>
    <ul>
      {{range .Errors}}<li><span class="badge badge-err">error</span> {{.}}</li>{{end}}
      {{range .ValidationErrors}}<li class="verr"><span class="badge badge-err">{{.Type}}</span> [line {{.LastLine}}] {{.Message}}</li>{{end}}
    </ul>
  </td>
</tr>
{{end}}
</tbody>
</table>
{{else}}
<p class="ok">No issues found.</p>
{{end}}
</body>
</html>`

	t := template.Must(template.New("report").Parse(tmpl))
	_ = t.Execute(os.Stdout, d)
}

func reportCSV(results []result) {
	w := csv.NewWriter(os.Stdout)

	_ = w.Write([]string{"url", "status", "redirect", "referrers", "errors"})

	for _, r := range results {
		if r.StatusCode == 200 && len(r.Errors) == 0 && len(r.ValidationErrors) == 0 && r.Redirect == "" {
			continue
		}

		var errs []string
		errs = append(errs, r.Errors...)
		for _, ve := range r.ValidationErrors {
			errs = append(errs, fmt.Sprintf("[line %d] %s", ve.LastLine, strings.TrimSpace(ve.Message)))
		}

		_ = w.Write([]string{
			r.URL,
			strconv.Itoa(r.StatusCode),
			r.Redirect,
			strings.Join(referrers[r.URL], "\n"),
			strings.Join(errs, "\n"),
		})
	}

	w.Flush()
}
