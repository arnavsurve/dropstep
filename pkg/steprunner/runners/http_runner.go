package runners

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/arnavsurve/dropstep/pkg/steprunner"
	"github.com/arnavsurve/dropstep/pkg/types"
)

const defaultHttpTimeout = 30 * time.Second

type HttpRunner struct {
	StepCtx types.ExecutionContext
}

func init() {
	steprunner.RegisterRunnerFactory("http", func(ctx types.ExecutionContext) (steprunner.StepRunner, error) {
		return &HttpRunner{
			StepCtx: ctx,
		}, nil
	})
}

func (hr *HttpRunner) Validate() error {
	step := hr.StepCtx.Step
	logger := hr.StepCtx.Logger

	if step.Call == nil {
		return fmt.Errorf("http step %q must define 'call'", step.ID)
	}

	if step.Call.Method == "" {
		return fmt.Errorf("http step %q: 'call.method' is required", step.ID)
	}
	// TODO: Basic validation for HTTP method, can be expanded
	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true, "HEAD": true, "OPTIONS": true,
	}
	if !validMethods[strings.ToUpper(step.Call.Method)] {
		logger.Warn().Str("method", step.Call.Method).Msg("Non-standard HTTP method specified. Proceeding, but ensure server supports it.")
	}

	if step.Call.Url == "" {
		return fmt.Errorf("http step %q: 'call.url' is required", step.ID)
	}

	if step.BrowserConfig.Prompt != "" {
		return fmt.Errorf("http step %q must not define 'browser.prompt'", step.ID)
	}
	if step.Command != nil {
		return fmt.Errorf("http step %q must not define 'run'", step.ID)
	}
	if step.BrowserConfig.UploadFiles != nil {
		return fmt.Errorf("http step %q must not define 'browser.upload_files'", step.ID)
	}
	if step.BrowserConfig.TargetDownloadDir != "" {
		return fmt.Errorf("http step %q must not define 'browser.download_dir'", step.ID)
	}
	if step.BrowserConfig.OutputSchemaFile != "" {
		return fmt.Errorf("http step %q must not define 'browser.output_schema'", step.ID)
	}
	if step.Provider != "" {
		return fmt.Errorf("http step %q must not define 'provider'", step.ID)
	}
	if step.BrowserConfig.AllowedDomains != nil {
		return fmt.Errorf("http step %q must not define 'browser.allowed_domains'", step.ID)
	}
	if step.BrowserConfig.MaxSteps != nil {
		return fmt.Errorf("http step %q must not define 'browser.max_steps'", step.ID)
	}
	if step.MaxFailures != nil {
		return fmt.Errorf("http step %q must not define 'max_failures'", step.ID)
	}

	return nil
}

func (hr *HttpRunner) Run() (*types.StepResult, error) {
	step := hr.StepCtx.Step
	logger := hr.StepCtx.Logger

	callDetails := step.Call
	method := strings.ToUpper(callDetails.Method)
	url := callDetails.Url

	var reqBody io.Reader
	var reqBodyBytes []byte
	if callDetails.Body != nil && (method == "POST" || method == "PUT" || method == "PATCH") {
		jsonBody, err := json.Marshal(callDetails.Body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body to JSON: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
		reqBodyBytes = jsonBody
	}

	// Prepare request
	timeout := defaultHttpTimeout
	if step.Timeout != "" {
		parsedDuration, err := time.ParseDuration(step.Timeout)
		if err != nil {
			logger.Warn().Err(err).Str("timeout", step.Timeout).Msg("Failed to parse timeout duration, using default")
		} else {
			timeout = parsedDuration
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}

	// Set headers
	hasContentType := false
	for key, value := range callDetails.Headers {
		req.Header.Set(key, value)
		if strings.ToLower(key) == "content-type" {
			hasContentType = true
		}
	}
	if reqBody != nil && !hasContentType {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "Dropstep-Http-Client/1.0")

	// Log request (redacted)
	// Redaction of headers/body will happen at the logger sink level
	logger.Info().
		Str("method", method).
		Str("url", url).
		Interface("headers", callDetails.Headers).
		Msg("Making HTTP request")
	if len(reqBodyBytes) > 0 {
		// Log a preview of the body if it's small, or just its presence
		bodyLog := string(reqBodyBytes)
		if len(bodyLog) > 256 {
			bodyLog = bodyLog[:256] + "..."
		}
		logger.Debug().Str("body_preview", bodyLog).Msg("Request body (redacted)")
	}

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	logger.Info().
		Int("status_code", resp.StatusCode).
		Interface("response_headers", resp.Header).
		Msg("Received HTTP response")
	if len(respBodyBytes) > 0 {
		bodyLog := string(respBodyBytes)
		if len(bodyLog) > 256 {
			bodyLog = bodyLog[:256] + "..."
		}
		logger.Debug().Str("body_preview", bodyLog).Msg("Response body (redacted)")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Warn().Int("status_code", resp.StatusCode).Msg("Received non-success HTTP response (non-2xx)")
	}

	output := make(map[string]any)
	output["status_code"] = resp.StatusCode

	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		respHeaders[k] = strings.Join(v, ", ")
	}
	output["headers"] = respHeaders

	var responseOutputBody any
	var parsedJsonAttempt any

	if errUnmarshal := json.Unmarshal(respBodyBytes, &parsedJsonAttempt); errUnmarshal == nil {
		responseOutputBody = parsedJsonAttempt
	} else {
		if utf8Str := string(respBodyBytes); strings.ToValidUTF8(utf8Str, "") == utf8Str {
			responseOutputBody = utf8Str
		} else {
			responseOutputBody = base64.StdEncoding.EncodeToString(respBodyBytes)
			logger.Warn().
				Int("body_size_bytes", len(respBodyBytes)).
				Msg("Response body was not valid JSON nor UTF-8 string, storing as base64.")
		}
	}
	output["body"] = responseOutputBody

	return &types.StepResult{Output: output}, nil
}
