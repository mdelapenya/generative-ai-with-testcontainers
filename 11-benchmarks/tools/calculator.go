package tools

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/tmc/langchaingo/llms"
)

// CalculatorInput represents the input parameters for calculator operations
type CalculatorInput struct {
	Operation string  `json:"operation"` // add, subtract, multiply, divide, power, sqrt, factorial
	A         float64 `json:"a"`
	B         float64 `json:"b,omitempty"` // Optional for unary operations like sqrt, factorial
}

// CalculatorResult represents the result of a calculator operation
type CalculatorResult struct {
	Result float64 `json:"result"`
	Error  string  `json:"error,omitempty"`
}

// Calculator implements basic mathematical operations as a tool for LLMs
type Calculator struct{}

// NewCalculator creates a new calculator tool
func NewCalculator() *Calculator {
	return &Calculator{}
}

// Execute performs a mathematical operation based on the input
func (c *Calculator) Execute(inputJSON string) (string, error) {
	var input CalculatorInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return "", fmt.Errorf("failed to parse calculator input: %w", err)
	}

	var result float64
	var err error

	switch input.Operation {
	case "add":
		result = input.A + input.B
	case "subtract":
		result = input.A - input.B
	case "multiply":
		result = input.A * input.B
	case "divide":
		if input.B == 0 {
			err = fmt.Errorf("division by zero")
		} else {
			result = input.A / input.B
		}
	case "power":
		result = math.Pow(input.A, input.B)
	case "sqrt":
		if input.A < 0 {
			err = fmt.Errorf("square root of negative number")
		} else {
			result = math.Sqrt(input.A)
		}
	case "factorial":
		if input.A < 0 || input.A != math.Floor(input.A) {
			err = fmt.Errorf("factorial requires non-negative integer")
		} else {
			result = float64(factorial(int(input.A)))
		}
	default:
		err = fmt.Errorf("unknown operation: %s", input.Operation)
	}

	calcResult := CalculatorResult{
		Result: result,
	}
	if err != nil {
		calcResult.Error = err.Error()
	}

	resultJSON, jsonErr := json.Marshal(calcResult)
	if jsonErr != nil {
		return "", fmt.Errorf("failed to marshal calculator result: %w", jsonErr)
	}

	return string(resultJSON), err
}

// GetToolDefinition returns the langchaingo tool definition for the calculator
func (c *Calculator) GetToolDefinition() llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "calculator",
			Description: "Performs basic mathematical operations (add, subtract, multiply, divide, power, sqrt, factorial). Use this tool when you need to perform calculations.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"operation": map[string]any{
						"type": "string",
						"enum": []string{"add", "subtract", "multiply", "divide", "power", "sqrt", "factorial"},
						"description": "The mathematical operation to perform",
					},
					"a": map[string]any{
						"type":        "number",
						"description": "The first operand (or the only operand for sqrt and factorial)",
					},
					"b": map[string]any{
						"type":        "number",
						"description": "The second operand (required for binary operations: add, subtract, multiply, divide, power)",
					},
				},
				"required": []string{"operation", "a"},
			},
		},
	}
}

// factorial calculates the factorial of a non-negative integer
func factorial(n int) int {
	if n == 0 || n == 1 {
		return 1
	}
	result := 1
	for i := 2; i <= n; i++ {
		result *= i
	}
	return result
}
