package amazon_agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"browser-agent/internal/browser"
	"browser-agent/internal/llm"
)

type Validator struct {
	llm *llm.GeminiClient
}

type ValidationResult struct {
	IsComplete       bool   `json:"is_complete"`
	NeedsReplanning  bool   `json:"needs_replanning"`
	Message          string `json:"message"`
	Confidence       float64 `json:"confidence"`
}

func NewValidator(llmClient *llm.GeminiClient) *Validator {
	return &Validator{llm: llmClient}
}

func (v *Validator) ValidateProgress(ctx *ExecutionContext, pageState *browser.PageState) (*ValidationResult, error) {
	executedStepsDesc := ""
	for i, step := range ctx.ExecutedSteps {
		status := "✓"
		if !step.Success {
			status = "✗"
		}
		executedStepsDesc += fmt.Sprintf("%d. %s %s\n", i+1, status, step.Step.Description)
	}

	remainingStepsDesc := ""
	for i := ctx.CurrentStepNum + 1; i < len(ctx.Plan.Steps) && i < ctx.CurrentStepNum+5; i++ {
		remainingStepsDesc += fmt.Sprintf("%d. %s\n", i+1, ctx.Plan.Steps[i].Description)
	}

	contentPreview := pageState.Content
	if len(contentPreview) > 2000 {
		contentPreview = contentPreview[:2000] + "..."
	}

	prompt := fmt.Sprintf(`You are validating browser automation progress.

Original Task: %s

Executed Steps:
%s

Remaining Planned Steps:
%s

Current Page State:
- URL: %s
- Title: %s
- Content Preview: %s

Analyze if:
1. The task is complete (goal achieved)
2. Progress is stuck or going wrong (needs replanning)
3. Everything is progressing normally (continue as planned)

Return ONLY valid JSON:
{
  "is_complete": true/false,
  "needs_replanning": true/false,
  "message": "explanation",
  "confidence": 0.0-1.0
}`, ctx.TaskDescription, executedStepsDesc, remainingStepsDesc, pageState.URL, pageState.Title, contentPreview)

	response, err := v.llm.Generate(prompt)
	if err != nil {
		return &ValidationResult{
			IsComplete:      false,
			NeedsReplanning: false,
			Message:         "Validation unavailable, continuing",
			Confidence:      0.5,
		}, nil
	}

	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}

	var result ValidationResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return &ValidationResult{
			IsComplete:      false,
			NeedsReplanning: false,
			Message:         "Continuing with plan",
			Confidence:      0.5,
		}, nil
	}

	return &result, nil
}