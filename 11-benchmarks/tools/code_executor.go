package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tmc/langchaingo/llms"
)

// CodeExecutorInput represents the input parameters for code execution
type CodeExecutorInput struct {
	Code     string `json:"code"`               // Python code to execute
	Language string `json:"language,omitempty"` // Programming language (default: python)
}

// CodeExecutorResult represents the result of code execution
type CodeExecutorResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// CodeExecutor executes code in isolated containers
type CodeExecutor struct {
	timeout time.Duration // Execution timeout
}

// NewCodeExecutor creates a new code executor tool with a default timeout
func NewCodeExecutor() *CodeExecutor {
	return &CodeExecutor{
		timeout: 30 * time.Second, // Default 30 second timeout
	}
}

// NewCodeExecutorWithTimeout creates a new code executor with a custom timeout
func NewCodeExecutorWithTimeout(timeout time.Duration) *CodeExecutor {
	return &CodeExecutor{
		timeout: timeout,
	}
}

// Execute runs the provided code in an isolated Python container
func (c *CodeExecutor) Execute(inputJSON string) (string, error) {
	var input CodeExecutorInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return "", fmt.Errorf("failed to parse code executor input: %w", err)
	}

	// Default to Python if language not specified
	if input.Language == "" {
		input.Language = "python"
	}

	// Only support Python for now
	if input.Language != "python" {
		result := CodeExecutorResult{
			Error:    fmt.Sprintf("unsupported language: %s (only python is supported)", input.Language),
			ExitCode: 1,
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON), fmt.Errorf("unsupported language: %s", input.Language)
	}

	// Execute Python code
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	stdout, stderr, exitCode, err := c.executePythonCode(ctx, input.Code)

	result := CodeExecutorResult{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}
	if err != nil {
		result.Error = err.Error()
	}

	resultJSON, jsonErr := json.Marshal(result)
	if jsonErr != nil {
		return "", fmt.Errorf("failed to marshal code executor result: %w", jsonErr)
	}

	return string(resultJSON), err
}

// executePythonCode runs Python code in a container and returns stdout, stderr, and exit code
func (c *CodeExecutor) executePythonCode(ctx context.Context, code string) (string, string, int, error) {
	// Create a Python container with the code as a command
	req := testcontainers.ContainerRequest{
		Image: "python:3.12-alpine",
		Cmd:   []string{"python", "-c", code},
		WaitingFor: wait.ForExit().
			WithExitTimeout(c.timeout),
	}

	// Start the container
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return "", "", 1, fmt.Errorf("failed to start Python container: %w", err)
	}
	defer func() {
		// Terminate the container
		if termErr := container.Terminate(ctx); termErr != nil {
			// Log but don't fail on cleanup errors
			fmt.Printf("Warning: failed to terminate container: %v\n", termErr)
		}
	}()

	// Wait for container to finish and get exit code
	exitCode, err := container.State(ctx)
	if err != nil {
		return "", "", 1, fmt.Errorf("failed to get container state: %w", err)
	}

	// Get stdout
	stdout, err := container.Logs(ctx)
	if err != nil {
		return "", "", exitCode.ExitCode, fmt.Errorf("failed to get container logs: %w", err)
	}
	defer stdout.Close()

	// Read stdout
	var stdoutBuilder strings.Builder
	buf := make([]byte, 1024)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			stdoutBuilder.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	// Container logs include both stdout and stderr combined
	// For simplicity, we'll put it all in stdout
	// A more sophisticated implementation could separate them
	stdoutStr := stdoutBuilder.String()

	// Check for execution errors
	var execErr error
	if exitCode.ExitCode != 0 {
		execErr = fmt.Errorf("code execution failed with exit code %d", exitCode.ExitCode)
	}

	return stdoutStr, "", exitCode.ExitCode, execErr
}

// GetToolDefinition returns the langchaingo tool definition for the code executor
func (c *CodeExecutor) GetToolDefinition() llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "execute_python",
			Description: "Executes Python code in a secure, isolated container and returns the output. Use this tool to run Python code, test algorithms, or validate code correctness.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"code": map[string]any{
						"type":        "string",
						"description": "The Python code to execute",
					},
					"language": map[string]any{
						"type":        "string",
						"enum":        []string{"python"},
						"description": "The programming language (currently only 'python' is supported)",
					},
				},
				"required": []string{"code"},
			},
		},
	}
}
