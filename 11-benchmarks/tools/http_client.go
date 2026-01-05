package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
)

// HTTPClientInput represents the input parameters for HTTP requests
type HTTPClientInput struct {
	URL     string            `json:"url"`               // URL to fetch
	Method  string            `json:"method,omitempty"`  // HTTP method (GET, POST, etc.)
	Headers map[string]string `json:"headers,omitempty"` // Optional headers
	Body    string            `json:"body,omitempty"`    // Optional request body for POST
}

// HTTPClientResult represents the result of an HTTP request
type HTTPClientResult struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body"`
	Error      string            `json:"error,omitempty"`
}

// HTTPClient makes HTTP requests to external APIs
type HTTPClient struct {
	client  *http.Client
	timeout time.Duration
}

// NewHTTPClient creates a new HTTP client tool with default timeout
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second, // Default 30 second timeout
		},
		timeout: 30 * time.Second,
	}
}

// NewHTTPClientWithTimeout creates a new HTTP client with a custom timeout
func NewHTTPClientWithTimeout(timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// Execute performs an HTTP request based on the input
func (h *HTTPClient) Execute(inputJSON string) (string, error) {
	var input HTTPClientInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return "", fmt.Errorf("failed to parse HTTP client input: %w", err)
	}

	// Default to GET if method not specified
	if input.Method == "" {
		input.Method = "GET"
	}

	// Validate HTTP method
	input.Method = strings.ToUpper(input.Method)
	if input.Method != "GET" && input.Method != "POST" {
		result := HTTPClientResult{
			Error:      fmt.Sprintf("unsupported HTTP method: %s (only GET and POST are supported)", input.Method),
			StatusCode: 0,
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON), fmt.Errorf("unsupported HTTP method: %s", input.Method)
	}

	// Validate URL
	if input.URL == "" {
		result := HTTPClientResult{
			Error:      "URL is required",
			StatusCode: 0,
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON), fmt.Errorf("URL is required")
	}

	// Create HTTP request
	var req *http.Request
	var err error

	if input.Method == "POST" && input.Body != "" {
		req, err = http.NewRequest(input.Method, input.URL, strings.NewReader(input.Body))
	} else {
		req, err = http.NewRequest(input.Method, input.URL, nil)
	}

	if err != nil {
		result := HTTPClientResult{
			Error:      fmt.Sprintf("failed to create HTTP request: %v", err),
			StatusCode: 0,
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON), fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add custom headers
	for key, value := range input.Headers {
		req.Header.Set(key, value)
	}

	// Set default User-Agent if not provided
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "LLM-Benchmark-Tool/1.0")
	}

	// Execute request
	resp, err := h.client.Do(req)
	if err != nil {
		result := HTTPClientResult{
			Error:      fmt.Sprintf("HTTP request failed: %v", err),
			StatusCode: 0,
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON), fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		result := HTTPClientResult{
			Error:      fmt.Sprintf("failed to read response body: %v", err),
			StatusCode: resp.StatusCode,
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON), fmt.Errorf("failed to read response body: %w", err)
	}

	// Extract response headers (only a few common ones to avoid verbosity)
	responseHeaders := make(map[string]string)
	for _, key := range []string{"Content-Type", "Content-Length", "Date"} {
		if value := resp.Header.Get(key); value != "" {
			responseHeaders[key] = value
		}
	}

	// Build result
	result := HTTPClientResult{
		StatusCode: resp.StatusCode,
		Headers:    responseHeaders,
		Body:       string(bodyBytes),
	}

	// Check for HTTP errors (4xx, 5xx)
	var execErr error
	if resp.StatusCode >= 400 {
		execErr = fmt.Errorf("HTTP request returned error status: %d", resp.StatusCode)
		result.Error = execErr.Error()
	}

	resultJSON, jsonErr := json.Marshal(result)
	if jsonErr != nil {
		return "", fmt.Errorf("failed to marshal HTTP client result: %w", jsonErr)
	}

	return string(resultJSON), execErr
}

// GetToolDefinition returns the langchaingo tool definition for the HTTP client
func (h *HTTPClient) GetToolDefinition() llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "http_get",
			Description: "Makes HTTP GET or POST requests to external APIs and returns the response. Use this tool to fetch data from web APIs, retrieve JSON data, or interact with external services.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{
						"type":        "string",
						"description": "The URL to request (must be a valid HTTP or HTTPS URL)",
					},
					"method": map[string]any{
						"type":        "string",
						"enum":        []string{"GET", "POST"},
						"description": "The HTTP method to use (default: GET)",
					},
					"headers": map[string]any{
						"type":        "object",
						"description": "Optional HTTP headers as key-value pairs",
					},
					"body": map[string]any{
						"type":        "string",
						"description": "Optional request body for POST requests",
					},
				},
				"required": []string{"url"},
			},
		},
	}
}
