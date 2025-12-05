# Evaluation Criteria

This directory contains evaluation criteria for assessing LLM response quality. Each test case has its own folder with two text files.

## File Structure

Each test case folder contains:

- **system_prompt.txt**: Instructions for the evaluator LLM
  - Defines the role (e.g., "expert programming evaluator")
  - Specifies JSON response format
  - Lists evaluation criteria
  - Explains response values (yes/no/unsure)

- **reference.txt**: Reference answer or expected behavior
  - Describes what a correct answer should contain
  - Provides context for evaluation
  - Used by the evaluator to assess correctness

## Current Test Cases

1. **code-explanation**: Evaluates explanations of Go code (Fibonacci)
2. **mathematical-operations**: Validates arithmetic calculations (sum 1-100)
3. **creative-writing**: Assesses humor in Fibonacci jokes
4. **factual-question**: Checks historical knowledge (Toledo translation movement)
5. **code-generation**: Verifies recursive Fibonacci function generation

## Adding New Test Cases

1. Create a new folder with the test case name (use kebab-case)

2. Create `system_prompt.txt` with evaluation instructions:
   - Define the evaluator role and expertise domain
   - **IMPORTANT**: Specify compact JSON response format requirement
   - Emphasize keeping all JSON fields concise (max 2-3 sentences each)
   - Instruct to summarize answers, NOT copy full text/code
   - List concrete, measurable evaluation criteria
   - Define clear boundaries for yes/no/unsure responses
   - Add "CRITICAL" reminders to keep JSON compact

3. Create `reference.txt` with:
   - Description of what a correct answer should contain
   - Key facts, concepts, or patterns to look for
   - Expected values or behaviors

4. Update evaluator.go:
   - Add your test case name to the testCases slice in GetEvaluationCriteria()

5. Add corresponding test case to bench_llm_test.go:
   - Add entry to the testCases variable with appropriate prompts

## Best Practices

### System Prompts

- Be specific about the domain expertise required
- **Always emphasize compact JSON format** with concise fields (max 2-3 sentences)
- Explicitly instruct: "brief summary" NOT "the full text"
- For code: "Do NOT copy code into JSON"
- List concrete, measurable criteria (3-5 points)
- Define clear boundaries for yes/no/unsure responses
- Keep language objective and professional
- Add "CRITICAL" warnings to reinforce JSON compactness

### References

- Provide enough detail for accurate evaluation
- Include key facts, concepts, or patterns
- Mention specific values when applicable (e.g., "5050" for sum)
- Explain reasoning or methodology when relevant
- Keep focused on essential elements

### Testing

After creating new evaluation criteria, run the standalone tests to verify file loading.

## Examples

### Good System Prompt
- Clearly states evaluator expertise
- **Emphasizes compact JSON with max 2-3 sentence fields**
- Instructs to summarize, not copy full answers/code
- Includes specific JSON format requirements
- Lists 3-5 concrete evaluation criteria
- Defines yes/no/unsure boundaries
- Adds "CRITICAL" reminders for compactness

### Poor System Prompt
- Vague instructions like "Check if the code is good"
- Missing JSON format specification
- No clear evaluation criteria
- **Allows copying full answers into JSON (causes truncation)**

### Good Reference
- Provides concrete example or detailed description
- Mentions specific values, patterns, or behaviors
- Explains key concepts that should be present

### Poor Reference
- Generic statements like "The code should work correctly"
- Missing specific details or examples

## Maintenance

- Review and update criteria based on evaluation results
- Adjust thresholds if too many "unsure" responses
- Refine language for better LLM understanding
- Keep files synced with test cases in code
