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
	IsComplete      bool    `json:"is_complete"`
	NeedsReplanning bool    `json:"needs_replanning"`
	Message         string  `json:"message"`
	Confidence      float64 `json:"confidence"`
	CurrentPhase    string  `json:"current_phase"`
}

func NewValidator(llmClient *llm.GeminiClient) *Validator {
	return &Validator{llm: llmClient}
}

func (v *Validator) ValidateProgress(ctx *ExecutionContext, pageState *browser.PageState) (*ValidationResult, error) {
	executedStepsDesc := ""
	successCount := 0
	for i, step := range ctx.ExecutedSteps {
		status := "✓"
		if !step.Success {
			status = "✗"
		} else {
			successCount++
		}
		executedStepsDesc += fmt.Sprintf("%d. %s %s\n", i+1, status, step.Step.Description)
	}

	remainingStepsDesc := ""
	for i := ctx.CurrentStepNum; i < len(ctx.Plan.Steps) && i < ctx.CurrentStepNum+5; i++ {
		remainingStepsDesc += fmt.Sprintf("%d. %s\n", i+1, ctx.Plan.Steps[i].Description)
	}

	contentPreview := pageState.Content
	if len(contentPreview) > 2000 {
		contentPreview = contentPreview[:2000] + "..."
	}

	memoryInfo := ""
	if ctx.Memory != nil {
		memoryInfo = fmt.Sprintf(`
Agent Memory:
- Products viewed: %d
- Selected product: %s
- Items in cart: %d
- User authenticated: %v
`, len(ctx.Memory.ProductURLs), ctx.Memory.SelectedProduct, len(ctx.Memory.CartItems), ctx.Memory.UserCredentials["email"] != "")
	}

	prompt := fmt.Sprintf(`You are validating complex browser automation progress for an e-commerce checkout flow.

Original Task: %s

Executed Steps (%d total, %d successful):
%s

%s

Next Planned Steps:
%s

Current Page State:
- URL: %s
- Title: %s
- Content Preview: %s

Analyze the progress and determine:
1. What phase are we in? (search/product_selection/cart/checkout/login/address/payment/complete)
2. Is the task fully complete? (reached payment confirmation screen)
3. Is progress stuck or going wrong? (needs replanning)
4. Is everything progressing normally? (continue)

Key completion indicators:
- For checkout tasks: Reached payment method selection or order review page
- URL contains: checkout, payment, order-review, place-order
- Page shows: payment options, order summary, final review

Return ONLY valid JSON:
{
  "is_complete": true/false,
  "needs_replanning": true/false,
  "message": "detailed explanation",
  "confidence": 0.0-1.0,
  "current_phase": "search|product_selection|cart|checkout|login|address|payment|complete"
}`, ctx.TaskDescription, len(ctx.ExecutedSteps), successCount, executedStepsDesc, memoryInfo, remainingStepsDesc, pageState.URL, pageState.Title, contentPreview)

	response, err := v.llm.Generate(prompt)
	if err != nil {
		return &ValidationResult{
			IsComplete:      false,
			NeedsReplanning: false,
			Message:         "Validation unavailable, continuing",
			Confidence:      0.5,
			CurrentPhase:    "unknown",
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
			CurrentPhase:    "unknown",
		}, nil
	}

	if result.CurrentPhase != "" {
		fmt.Printf("   📍 Current phase: %s\n", result.CurrentPhase)
	}

	return &result, nil
}