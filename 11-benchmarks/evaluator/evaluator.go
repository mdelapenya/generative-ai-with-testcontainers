package evaluator

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/tmc/langchaingo/llms"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

// EvaluationResult represents the structured response from the evaluator LLM
type EvaluationResult struct {
	ProvidedAnswer string  `json:"provided_answer"`
	Response       string  `json:"response"` // "yes", "no", or "unsure"
	Reason         string  `json:"reason"`
	Score          float64 `json:"score"` // 0.0 to 1.0
}

// ToolEvaluationResult represents the evaluation of tool calling accuracy
type ToolEvaluationResult struct {
	ToolSelectionScore float64 `json:"tool_selection_score"` // 0.0-1.0: correct tool chosen
	ParameterAccuracy  float64 `json:"parameter_accuracy"`   // 0.0-1.0: parameters match expected
	SequenceScore      float64 `json:"sequence_score"`       // 0.0-1.0: logical call order
	OverallScore       float64 `json:"overall_score"`        // Average of above
	Reason             string  `json:"reason"`               // Explanation
}

// Evaluator defines the interface for evaluating LLM responses
type Evaluator interface {
	Evaluate(ctx context.Context, testCase string, question string, answer string, reference string) (*EvaluationResult, error)
}

// Agent implements the Evaluator interface using an LLM as a judge
type Agent struct {
	systemMessage string
	chatModel     llms.Model
	userTemplate  string
}

// NewAgent creates a new evaluator agent with a specific system prompt
func NewAgent(model llms.Model, systemPrompt string) *Agent {
	userTemplate := `Question: %s
Answer: %s
Reference: %s
JSON response:`

	return &Agent{
		systemMessage: systemPrompt,
		chatModel:     model,
		userTemplate:  userTemplate,
	}
}

// Evaluate assesses the quality of an answer against a reference using the LLM judge
func (e *Agent) Evaluate(ctx context.Context, testCase string, question string, answer string, reference string) (*EvaluationResult, error) {
	// Construct the user message with the question, answer, and reference
	userMessage := fmt.Sprintf(e.userTemplate, question, answer, reference)

	// Create message content
	msgContent := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, e.systemMessage),
		llms.TextParts(llms.ChatMessageTypeHuman, userMessage),
	}

	// Generate response with deterministic parameters
	resp, err := e.chatModel.GenerateContent(ctx, msgContent,
		llms.WithTemperature(0.0),
		llms.WithTopK(1),
		llms.WithSeed(42),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate evaluation: %w", err)
	}

	// Extract response text
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned from evaluator")
	}

	var responseText string
	for _, choice := range resp.Choices {
		if choice.Content != "" {
			responseText += choice.Content
		}
	}

	// Try to extract JSON from the response
	// Sometimes the model may add extra text before/after the JSON
	jsonText := extractJSON(responseText)
	if jsonText == "" {
		return nil, fmt.Errorf("no JSON found in evaluation response (response: %s)", responseText)
	}

	// Parse JSON response
	var result EvaluationResult
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return nil, fmt.Errorf("failed to parse evaluation response as JSON: %w (response: %s)", err, jsonText)
	}

	// Convert response to score
	result.Score = responseToScore(result.Response)

	// Log the evaluation result
	logger := global.GetLoggerProvider().Logger("evaluator")
	var record log.Record
	record.SetSeverity(log.SeverityInfo)
	record.SetBody(log.StringValue("Evaluator response"))
	record.AddAttributes(
		log.String("test_case", sanitizeUTF8(testCase)),
		log.String("question", truncateString(question, 100)),
		log.String("answer", truncateString(answer, 200)),
		log.String("provided_answer", sanitizeUTF8(truncateString(result.ProvidedAnswer, 200))),
		log.String("response", sanitizeUTF8(result.Response)),
		log.String("reason", sanitizeUTF8(truncateString(result.Reason, 500))),
		log.Float64("score", result.Score),
	)
	logger.Emit(ctx, record)

	return &result, nil
}

// truncateString truncates a string to a maximum length and ensures valid UTF-8
func truncateString(s string, maxLen int) string {
	// First, sanitize to valid UTF-8
	s = sanitizeUTF8(s)

	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// sanitizeUTF8 replaces invalid UTF-8 sequences with the replacement character
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	// Replace invalid UTF-8 with valid replacement characters
	return strings.ToValidUTF8(s, "�")
}

// extractJSON attempts to extract a JSON object from a string
// It looks for the first '{' and the last '}' to handle cases where
// the model adds extra text before or after the JSON
// If the JSON appears incomplete (no closing }), it tries to fix it
// Also fixes common JSON formatting issues like unescaped tabs and newlines
func extractJSON(text string) string {
	startIdx := strings.Index(text, "{")
	if startIdx == -1 {
		return ""
	}

	endIdx := strings.LastIndex(text, "}")
	if endIdx == -1 || endIdx < startIdx {
		// JSON appears incomplete - try to fix it by adding a closing }
		// First, ensure the last field is properly closed
		jsonText := text[startIdx:]

		// If the text ends with a quote, it's likely the last string field
		// Add closing brace
		if strings.HasSuffix(strings.TrimSpace(jsonText), `"`) {
			jsonText = strings.TrimSpace(jsonText) + "\n}"
			return jsonText
		}

		// Otherwise, try to close any open quotes and add closing brace
		if strings.Count(jsonText, `"`)%2 != 0 {
			jsonText = jsonText + `"` + "\n}"
			return jsonText
		}

		// Last resort - just add closing brace
		jsonText = jsonText + "\n}"
		return fixJSONEscaping(jsonText)
	}

	jsonText := text[startIdx : endIdx+1]
	return fixJSONEscaping(jsonText)
}

// fixJSONEscaping fixes common JSON escaping issues in string values
// Handles unescaped tabs, newlines, and other control characters within quoted strings
func fixJSONEscaping(jsonText string) string {
	var result strings.Builder
	inString := false
	escaped := false

	for i := 0; i < len(jsonText); i++ {
		ch := jsonText[i]

		// Track if we're inside a string
		if ch == '"' && !escaped {
			inString = !inString
			result.WriteByte(ch)
			continue
		}

		// Track escape sequences
		if ch == '\\' && !escaped {
			escaped = true
			result.WriteByte(ch)
			continue
		}

		// If we were escaped, reset the flag
		if escaped {
			escaped = false
			result.WriteByte(ch)
			continue
		}

		// Only fix unescaped control characters inside strings
		if inString {
			switch ch {
			case '\t':
				result.WriteString("\\t")
			case '\r':
				result.WriteString("\\r")
			case '\n':
				result.WriteString("\\n")
			default:
				result.WriteByte(ch)
			}
		} else {
			result.WriteByte(ch)
		}
	}

	return result.String()
}

// responseToScore converts the text response ("yes"/"no"/"unsure") to a numeric score
func responseToScore(response string) float64 {
	switch strings.ToLower(strings.TrimSpace(response)) {
	case "yes":
		return 1.0
	case "no":
		return 0.0
	case "unsure":
		return 0.5
	default:
		return 0.0
	}
}

// Embedded evaluation criteria files
//
//go:embed testdata/evaluation/code-explanation/system_prompt.txt
var codeExplanationSystemPrompt string

//go:embed testdata/evaluation/code-explanation/reference.txt
var codeExplanationReference string

//go:embed testdata/evaluation/mathematical-operations/system_prompt.txt
var mathSystemPrompt string

//go:embed testdata/evaluation/mathematical-operations/reference.txt
var mathReference string

//go:embed testdata/evaluation/creative-writing/system_prompt.txt
var creativeWritingSystemPrompt string

//go:embed testdata/evaluation/creative-writing/reference.txt
var creativeWritingReference string

//go:embed testdata/evaluation/factual-question/system_prompt.txt
var factualQuestionSystemPrompt string

//go:embed testdata/evaluation/factual-question/reference.txt
var factualQuestionReference string

//go:embed testdata/evaluation/code-generation/system_prompt.txt
var codeGenerationSystemPrompt string

//go:embed testdata/evaluation/code-generation/reference.txt
var codeGenerationReference string

// Tool parameter extraction evaluation criteria
//go:embed testdata/evaluation/tool-parameter-extraction/calculator-reasoning/system_prompt.txt
var calculatorReasoningToolSystemPrompt string

//go:embed testdata/evaluation/tool-parameter-extraction/calculator-reasoning/reference.txt
var calculatorReasoningToolReference string

//go:embed testdata/evaluation/tool-parameter-extraction/code-validation/system_prompt.txt
var codeValidationToolSystemPrompt string

//go:embed testdata/evaluation/tool-parameter-extraction/code-validation/reference.txt
var codeValidationToolReference string

//go:embed testdata/evaluation/tool-parameter-extraction/api-data-retrieval/system_prompt.txt
var apiDataRetrievalToolSystemPrompt string

//go:embed testdata/evaluation/tool-parameter-extraction/api-data-retrieval/reference.txt
var apiDataRetrievalToolReference string

// Criteria defines the criteria for evaluating responses for different test cases
type Criteria struct {
	TestCaseName string
	SystemPrompt string
	Reference    string
}

// GetCriteria returns the evaluation criteria for each test case
// It uses embedded content from testdata/evaluation/{test-case-name}/
func GetCriteria() map[string]Criteria {
	return map[string]Criteria{
		"code-explanation": {
			TestCaseName: "code-explanation",
			SystemPrompt: strings.TrimSpace(codeExplanationSystemPrompt),
			Reference:    strings.TrimSpace(codeExplanationReference),
		},
		"mathematical-operations": {
			TestCaseName: "mathematical-operations",
			SystemPrompt: strings.TrimSpace(mathSystemPrompt),
			Reference:    strings.TrimSpace(mathReference),
		},
		"creative-writing": {
			TestCaseName: "creative-writing",
			SystemPrompt: strings.TrimSpace(creativeWritingSystemPrompt),
			Reference:    strings.TrimSpace(creativeWritingReference),
		},
		"factual-question": {
			TestCaseName: "factual-question",
			SystemPrompt: strings.TrimSpace(factualQuestionSystemPrompt),
			Reference:    strings.TrimSpace(factualQuestionReference),
		},
		"code-generation": {
			TestCaseName: "code-generation",
			SystemPrompt: strings.TrimSpace(codeGenerationSystemPrompt),
			Reference:    strings.TrimSpace(codeGenerationReference),
		},
		// Tool parameter extraction criteria
		"calculator-reasoning": {
			TestCaseName: "calculator-reasoning",
			SystemPrompt: strings.TrimSpace(calculatorReasoningToolSystemPrompt),
			Reference:    strings.TrimSpace(calculatorReasoningToolReference),
		},
		"code-validation": {
			TestCaseName: "code-validation",
			SystemPrompt: strings.TrimSpace(codeValidationToolSystemPrompt),
			Reference:    strings.TrimSpace(codeValidationToolReference),
		},
		"api-data-retrieval": {
			TestCaseName: "api-data-retrieval",
			SystemPrompt: strings.TrimSpace(apiDataRetrievalToolSystemPrompt),
			Reference:    strings.TrimSpace(apiDataRetrievalToolReference),
		},
	}
}

// EvaluateToolCalls evaluates the accuracy of tool calling in an LLM response
// It checks tool selection, parameter correctness, and call sequence
func (e *Agent) EvaluateToolCalls(ctx context.Context, testCase string, question string, answer string, reference string) (*ToolEvaluationResult, error) {
	// Use the same evaluation template but with tool-specific prompt
	userMessage := fmt.Sprintf(e.userTemplate, question, answer, reference)

	// Create message content
	msgContent := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, e.systemMessage),
		llms.TextParts(llms.ChatMessageTypeHuman, userMessage),
	}

	// Generate response with deterministic parameters
	resp, err := e.chatModel.GenerateContent(ctx, msgContent,
		llms.WithTemperature(0.0),
		llms.WithTopK(1),
		llms.WithSeed(42),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tool evaluation: %w", err)
	}

	// Extract response text
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned from evaluator")
	}

	var responseText string
	for _, choice := range resp.Choices {
		if choice.Content != "" {
			responseText += choice.Content
		}
	}

	// Try to extract JSON from the response
	jsonText := extractJSON(responseText)
	if jsonText == "" {
		return nil, fmt.Errorf("no JSON found in tool evaluation response (response: %s)", responseText)
	}

	// Parse JSON response
	var result ToolEvaluationResult
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return nil, fmt.Errorf("failed to parse tool evaluation response as JSON: %w (response: %s)", err, jsonText)
	}

	// Calculate overall score as average of individual scores
	result.OverallScore = (result.ToolSelectionScore + result.ParameterAccuracy + result.SequenceScore) / 3.0

	// Log the tool evaluation result
	logger := global.GetLoggerProvider().Logger("evaluator")
	var record log.Record
	record.SetSeverity(log.SeverityInfo)
	record.SetBody(log.StringValue("Tool evaluation response"))
	record.AddAttributes(
		log.String("test_case", sanitizeUTF8(testCase)),
		log.Float64("tool_selection_score", result.ToolSelectionScore),
		log.Float64("parameter_accuracy", result.ParameterAccuracy),
		log.Float64("sequence_score", result.SequenceScore),
		log.Float64("overall_score", result.OverallScore),
		log.String("reason", sanitizeUTF8(truncateString(result.Reason, 500))),
	)
	logger.Emit(ctx, record)

	return &result, nil
}
