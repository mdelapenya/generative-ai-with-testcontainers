package evaluator

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEvaluationCriteriaFiles verifies that all evaluation criteria files exist and are readable
func TestEvaluationCriteriaFiles(t *testing.T) {
	testCases := []string{
		"code-explanation",
		"mathematical-operations",
		"creative-writing",
		"factual-question",
		"code-generation",
	}

	for _, testCase := range testCases {
		t.Run(testCase, func(t *testing.T) {
			criteriaDir := filepath.Join("testdata", "evaluation", testCase)

			// Check system_prompt.txt exists and is readable
			systemPromptPath := filepath.Join(criteriaDir, "system_prompt.txt")
			systemPrompt, err := os.ReadFile(systemPromptPath)
			if err != nil {
				t.Fatalf("Failed to read system prompt: %v", err)
			}
			if len(systemPrompt) == 0 {
				t.Error("System prompt is empty")
			}

			// Check reference.txt exists and is readable
			referencePath := filepath.Join(criteriaDir, "reference.txt")
			reference, err := os.ReadFile(referencePath)
			if err != nil {
				t.Fatalf("Failed to read reference: %v", err)
			}
			if len(reference) == 0 {
				t.Error("Reference is empty")
			}

			t.Logf("✓ System prompt: %d bytes", len(systemPrompt))
			t.Logf("✓ Reference: %d bytes", len(reference))
		})
	}
}

// TestGetCriteria tests the GetCriteria function
func TestGetCriteria(t *testing.T) {
	criteria := GetCriteria()

	expectedTestCases := []string{
		"code-explanation",
		"mathematical-operations",
		"creative-writing",
		"factual-question",
		"code-generation",
	}

	// Verify all test cases are loaded
	for _, testCase := range expectedTestCases {
		c, ok := criteria[testCase]
		if !ok {
			t.Errorf("Missing evaluation criteria for test case: %s", testCase)
			continue
		}

		// Verify system prompt is not empty
		if c.SystemPrompt == "" {
			t.Errorf("Empty system prompt for test case: %s", testCase)
		}

		// Verify reference is not empty
		if c.Reference == "" {
			t.Errorf("Empty reference for test case: %s", testCase)
		}

		// Verify test case name matches
		if c.TestCaseName != testCase {
			t.Errorf("Test case name mismatch: expected %s, got %s", testCase, c.TestCaseName)
		}

		t.Logf("✓ Loaded criteria for %s (prompt: %d chars, ref: %d chars)",
			testCase, len(c.SystemPrompt), len(c.Reference))
	}

	// Verify we have exactly the expected number of criteria
	if len(criteria) != len(expectedTestCases) {
		t.Errorf("Expected %d criteria, got %d", len(expectedTestCases), len(criteria))
	}
}
