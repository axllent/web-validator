package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

// NuJSONStruct response
type NuJSONStruct struct {
	Messages []ValidationError `json:"messages"`
	Source   struct {
		Type     string `json:"type"`
		Encoding string `json:"encoding"`
		Code     string `json:"code"`
	} `json:"source"`
	Language string `json:"language"`
}

// ValidationError struct
type ValidationError struct {
	Type         string `json:"type"`
	LastLine     int    `json:"lastLine"`
	LastColumn   int    `json:"lastColumn"`
	FirstColumn  int    `json:"firstColumn"`
	Message      string `json:"message"`
	Extract      string `json:"extract"`
	HiliteStart  int    `json:"hiliteStart"`
	HiliteLength int    `json:"hiliteLength"`
}

// Validate will validate HTML & CSS with Nu Validator
func validate(output Result, body io.Reader, contentType string) Result {
	if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "text/css") {
		return output
	}

	if !validateHTML && strings.Contains(contentType, "text/html") {
		return output
	}

	if !validateCSS && strings.Contains(contentType, "text/css") {
		return output
	}

	req, err := http.NewRequest("POST", htmlValidator, body)

	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("User-Agent", "Web-validator")

	if output.Type != "" {
		req.Header.Set("Content-Type", contentType)
	} else {
		req.Header.Set("Content-Type", "text/html; charset=utf-8")
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		output.Errors = append(output.Errors, fmt.Sprintf("Validator: %s", err))
		results = append(results, output)
		return output
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		output.Errors = append(output.Errors, fmt.Sprintf("Validator: %s", err))
		results = append(results, output)
		return output
	}

	if res.StatusCode != 200 {
		output.Errors = append(output.Errors, fmt.Sprintf("Validator: %s returned a %d (%s) response", htmlValidator, res.StatusCode, http.StatusText(res.StatusCode)))
		results = append(results, output)
		return output
	}

	response := NuJSONStruct{}
	jsonErr := json.Unmarshal(data, &response)
	if jsonErr != nil {
		output.Errors = append(output.Errors, fmt.Sprintf("Error parsing response from %s: %s", htmlValidator, string(data)))
		return output
	}

	for _, msg := range response.Messages {
		if msg.Type == "error" || (showWarnigs && msg.Type == "info") {
			output.ValidationErrors = append(output.ValidationErrors, msg)
			errorsProcessed++
		}
	}

	return output
}
