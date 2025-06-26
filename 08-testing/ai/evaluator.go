package ai

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
)

const (
	// systemPrompt is the system message for the evaluator, which instructs it to validate the answer
	// based on the question and reference. The prompt follows some instructions to validate the answer,
	// such as responding with 'yes', 'no' or 'unsure' and always including the reason for your response.
	// It also instructs the evaluator to respond with a json object with the following structure:
	// {
	// 	"response": "yes",
	// 	"reason": "The answer is correct because it is based on the reference provided."
	// }
	systemPrompt string = `
You are a strict validator that responds ONLY with valid JSON.
You will be provided with a question, an answer, and a reference.
Your task is to validate whether the answer is correct for the given question, based on the reference.

Follow these instructions:
- Respond only 'yes', 'no' or 'unsure' and always include the reason for your response
- Respond with 'yes' if the answer is correct
- Respond with 'no' if the answer is incorrect
- If you are unsure, simply respond with 'unsure'
- Respond with 'no' if the answer is not clear or concise
- Respond with 'no' if the answer is not based on the reference

CRITICAL: Return exactly ONE valid JSON object with no additional text or objects.

Your response must be a single valid json object with the following fields:
- "provided_answer": copy the exact text of the answer that was provided to you
- "response": "yes" or "no" or "unsure",
- "reason": the motivation for your response.

Example User input:
Question: Is Madrid the capital of Spain?
Answer: No, it's Barcelona.
Reference: The capital of Spain is Madrid

Here you can find an example of a valid JSON response to build your answer:
{
	"provided_answer": "No, it's Barcelona.",
	"response": "no",
	"reason": "The answer is incorrect because the reference states that the capital of Spain is Madrid."
}
`

	// userPrompt is the prompt for the user message.
	userPrompt string = `
Question: %s
Answer: %s
Reference: %s

JSON response:
`
)

type Evaluator interface {
	Evaluate(question string, answer string, reference string) (string, error)
}

type EvaluatorAgent struct {
	systemMessage string
	chatModel     llms.Model
	userMessage   string
}

func (v *EvaluatorAgent) Evaluate(question string, answer string, reference string) (string, error) {
	ctx := context.Background()
	content := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, v.systemMessage),
		llms.TextParts(llms.ChatMessageTypeHuman, fmt.Sprintf(v.userMessage, question, answer, reference)),
	}

	completion, err := v.chatModel.GenerateContent(
		ctx, content,
		llms.WithTemperature(0.00),
		llms.WithTopK(1),
		llms.WithSeed(42),
	)
	if err != nil {
		return "", fmt.Errorf("llm generate content: %w", err)
	}

	response := ""
	for _, choice := range completion.Choices {
		response += choice.Content
	}

	return response, nil
}

func NewEvaluatorAgent(model llms.Model) *EvaluatorAgent {
	v := &EvaluatorAgent{
		chatModel:     model,
		systemMessage: systemPrompt,
		userMessage:   userPrompt,
	}

	return v
}
