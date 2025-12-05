# LLM Evaluator Agent

This benchmark suite includes an intelligent evaluator agent that uses LLMs to assess the quality of responses from the models being benchmarked.

## Overview

The evaluator agent acts as an LLM judge, providing objective assessment of response quality based on specific criteria for each test case. This approach offers:

- **Automated Quality Assessment**: No manual review needed
- **Consistency**: Same criteria applied across all models
- **Detailed Feedback**: Understand why responses succeed or fail
- **Scoring**: Quantitative metrics (0.0-1.0) for comparison

## ⚠️ Evaluator Model Recommendation

**We strongly recommend using a high-quality LLM for the evaluator agent**, such as OpenAI's GPT-4 or GPT-4o-mini, rather than smaller local models.

### Why Use a High-Quality Evaluator?

The evaluator's job is to assess the correctness and quality of responses from the models being benchmarked (which may include small language models or SLMs). Using a high-quality evaluator ensures:

1. **Accurate Assessments**: High-quality models like GPT-4 have better reasoning capabilities and can more reliably judge response correctness
2. **Consistent JSON Generation**: Premium models are better at following structured output requirements and generating valid JSON
3. **Nuanced Evaluation**: Better understanding of edge cases, partial correctness, and quality gradations
4. **Reduced False Positives/Negatives**: More accurate distinction between correct, incorrect, and ambiguous responses
5. **Reliable SLM Benchmarking**: When benchmarking small/local models, you want confidence that evaluation accuracy isn't a bottleneck

### Recommended Configuration

```bash
# Set OpenAI API key to use GPT-4o-mini as evaluator (recommended)
export OPENAI_API_KEY="your-key-here"

# Run benchmarks - evaluator will automatically use GPT-4o-mini
go test -bench=. -benchmem -benchtime=3x
```

**Cost Consideration**: GPT-4o-mini is cost-effective for evaluation (much cheaper than GPT-4) while maintaining high accuracy. The added API cost is typically negligible compared to the value of accurate evaluation data.

### Local Evaluator Limitations

While the system falls back to local models (e.g., `ai/llama3.2:3B-Q4_K_M`) when no API key is available, be aware that:
- Local SLMs may struggle with strict JSON formatting requirements
- Evaluation accuracy may be lower, especially for nuanced test cases
- You may see more evaluation errors or inconsistent scoring
- This can undermine the reliability of your benchmark results

**Bottom Line**: For production benchmarking or important quality assessments, invest in using a high-quality evaluator model to ensure your SLM benchmarks are truly accurate and actionable.

## How It Works

### 1. Evaluator Agent Architecture

The evaluator uses **langchaingo** to create an LLM-powered judge that:
- Receives the original question, the model's answer, and reference criteria
- Applies test-case-specific evaluation prompts
- Returns structured JSON with:
  - `response`: "yes" (correct), "no" (incorrect), or "unsure" (ambiguous)
  - `reason`: Explanation of the evaluation
  - `score`: Numeric score (1.0 for yes, 0.0 for no, 0.5 for unsure)

### 2. Evaluation Criteria

Each test case has tailored evaluation criteria:

#### Code Explanation
- Identifies Fibonacci implementation
- Explains recursive approach
- Mentions base cases
- Clarity and accuracy

#### Mathematical Operations
- Correct result (5050 for sum 1-100)
- Proper calculation method
- Understanding of arithmetic series

#### Creative Writing
- Contains Fibonacci-related joke
- Demonstrates humor/wit
- Coherent and complete
- References Fibonacci concepts

#### Factual Question
- Mentions Toledo School of Translators
- References Arabic/Greek → Latin translations
- Notes 12th-13th century timeframe
- Explains cultural significance

#### Code Generation
- Valid Go code
- Implements Fibonacci with recursion
- Correct base cases
- Proper recursive calls

### 3. Evaluator Model Selection

The system intelligently selects the evaluator model:

**With OpenAI API Key (Recommended)**:
```bash
export OPENAI_API_KEY="your-key-here"
```
Uses `gpt-4o-mini` for evaluation (fast, cost-effective, highly accurate)

**Without API Key**:
Falls back to local model via Docker Model Runner: `ai/llama3.2:3B-Q4_K_M`

## Using the Evaluator

### Running Benchmarks with Evaluation

Simply run the benchmarks normally:

```bash
go test -bench=. -benchmem -benchtime=3x
```

The evaluator automatically:
1. Initializes during `TestMain`
2. Evaluates each response after generation
3. Adds evaluation scores to metrics
4. Reports aggregate evaluation scores

### Metrics Reported

New metrics added to benchmark output:

- `eval_score`: Average evaluation score (0.0-1.0) across all iterations
- Individual result fields:
  - `EvalScore`: Numeric score for the response
  - `EvalResponse`: "yes", "no", or "unsure"
  - `EvalReason`: Detailed explanation

### Example Output

```
BenchmarkLLMs/llama3.2/code-explanation/temp0.5-8
  200  5432109 ns/op  123.45 latency_p50_ms  0.85 eval_score
```

The `eval_score` of 0.85 indicates that 85% of responses met the criteria.

## Implementation Details

### Files

- **evaluator.go**: Core evaluator implementation
  - `EvaluatorAgent`: LLM judge implementation
  - `EvaluationCriteria`: Test-case-specific criteria
  - `GetEvaluationCriteria()`: Returns criteria from embedded files
  - Uses `go:embed` to embed evaluation files at compile time

- **testdata/evaluation/**: Evaluation criteria (embedded at build time)
  - Each test case has its own folder
  - `system_prompt.txt`: Instructions for the evaluator LLM (embedded via `//go:embed`)
  - `reference.txt`: Reference answer or expected behavior (embedded via `//go:embed`)

- **bench_llm_test.go**: Integration
  - `evaluateResponse()`: Evaluates a single response
  - Updated `BenchmarkResult` struct with eval fields
  - Enhanced `reportAggregateMetrics()` with eval scoring

- **bench_main_test.go**: Initialization
  - `initializeEvaluatorAgent()`: Sets up evaluator model
  - Global `evaluatorAgent` variable

### Customization

To add new test cases with evaluation:

1. Add test case to `testCases` in `bench_llm_test.go`
2. Create a new folder in `testdata/evaluation/{test-case-name}/`
3. Add two files in the folder:

**system_prompt.txt**:
```
You are an expert evaluator...

IMPORTANT: Respond ONLY with valid, compact JSON. Keep all fields concise (max 2-3 sentences each).

Required JSON format:
{
  "provided_answer": "brief summary of the answer (NOT the full text)",
  "response": "yes/no/unsure",
  "reason": "1-2 sentence explanation of your evaluation"
}

Evaluation criteria:
- Point 1
- Point 2

Response must be:
- "yes" if the answer meets criteria
- "no" if the answer fails criteria
- "unsure" if the answer is ambiguous

CRITICAL: Keep the JSON compact. Summarize answers briefly. Do NOT copy full text or code.
```

**reference.txt**:
```
Expected correct answer or behavior description.
```

4. Add `go:embed` directives in `evaluator.go`:
   ```go
   //go:embed testdata/evaluation/your-test-case/system_prompt.txt
   var yourTestCaseSystemPrompt string

   //go:embed testdata/evaluation/your-test-case/reference.txt
   var yourTestCaseReference string
   ```

5. Update `GetEvaluationCriteria()` in `evaluator.go` to include your test case:
   ```go
   "your-test-case": {
       TestCaseName: "your-test-case",
       SystemPrompt: strings.TrimSpace(yourTestCaseSystemPrompt),
       Reference:    strings.TrimSpace(yourTestCaseReference),
   },
   ```

### File Structure

```
testdata/evaluation/
├── code-explanation/
│   ├── system_prompt.txt
│   └── reference.txt
├── mathematical-operations/
│   ├── system_prompt.txt
│   └── reference.txt
├── creative-writing/
│   ├── system_prompt.txt
│   └── reference.txt
├── factual-question/
│   ├── system_prompt.txt
│   └── reference.txt
└── code-generation/
    ├── system_prompt.txt
    └── reference.txt
```

## Benefits

1. **Objective Quality Metrics**: Beyond latency and tokens, measure actual correctness
2. **Model Comparison**: Compare models on correctness, not just speed
3. **Temperature Analysis**: See how temperature affects quality
4. **Automated Testing**: No manual verification needed
5. **Continuous Improvement**: Track quality improvements over time
6. **Embedded Criteria**: Using `go:embed`, evaluation criteria is compiled into the binary
   - No runtime file I/O needed
   - Binary is self-contained and portable
   - Faster loading at runtime
   - No risk of missing files in deployment

## Limitations

- **Evaluator quality depends on the judge LLM capabilities** (see [Evaluator Model Recommendation](#️-evaluator-model-recommendation) above)
- Local evaluator models may be less accurate than GPT-4 or GPT-4o-mini
- Evaluation adds overhead (extra LLM calls per response being evaluated)
- JSON parsing can fail with poorly-formatted responses from smaller models

## Best Practices

1. **Use a high-quality evaluator model (OpenAI GPT-4o-mini or GPT-4) for accurate evaluations** - This is critical for reliable SLM benchmarking
2. Run multiple iterations (benchtime=5x or more) for statistical significance
3. Review `EvalReason` field to understand failure patterns and improve test criteria
4. Adjust evaluation criteria based on your specific requirements
5. Consider evaluator latency in overall benchmark time (evaluation adds overhead)
6. Monitor evaluation error rates - high error rates may indicate evaluator model issues

## Future Enhancements

Potential improvements:

- Support for additional evaluator models (Anthropic Claude, etc.)
- Multi-dimensional scoring (correctness, completeness, style)
- Caching evaluations to reduce API costs
- Comparative evaluation (A/B response comparison)
- Human-in-the-loop validation
