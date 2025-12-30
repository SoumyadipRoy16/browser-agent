package amazon_agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"browser-agent/internal/browser"
	"browser-agent/internal/llm"
)

type Planner struct {
	llm *llm.GeminiClient
}

type Plan struct {
	Steps []Step `json:"steps"`
}

type Step struct {
	Action      string            `json:"action"`
	Description string            `json:"description"`
	Target      string            `json:"target,omitempty"`
	Value       interface{}       `json:"value,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Critical    bool              `json:"critical"`
}

func (s *Step) GetValueString() string {
	if s.Value == nil {
		return ""
	}

	switch v := s.Value.(type) {
	case string:
		return v
	case float64:
		if v < 30000 && v == float64(int(v)) {
			return fmt.Sprintf("%.0fms", v)
		}
		return fmt.Sprintf("%v", v)
	case int:
		if v < 30000 {
			return fmt.Sprintf("%dms", v)
		}
		return fmt.Sprintf("%d", v)
	case bool:
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

type ExecutionContext struct {
	TaskDescription string
	Plan            *Plan
	ExecutedSteps   []ExecutedStep
	CurrentStepNum  int
	Memory          *AgentMemory
}

type ExecutedStep struct {
	Step      Step
	Success   bool
	Error     error
	Timestamp interface{}
}

func NewPlanner(llmClient *llm.GeminiClient) *Planner {
	return &Planner{llm: llmClient}
}

func (p *Planner) CreatePlan(taskDescription string) (*Plan, error) {
	prompt := fmt.Sprintf(`You are an advanced browser automation planner for complex e-commerce tasks. Create a comprehensive step-by-step plan for this task:

Task: %s

You must generate a detailed plan with 30-50+ steps that covers:
1. Navigation and initial setup
2. Search operations with proper selectors
3. Product browsing and selection with specific criteria
4. Scrolling to view more products if needed
5. Detailed product inspection
6. Add to cart operations
7. Cart verification
8. Checkout process initiation
9. Login/authentication handling (IMPORTANT: use "login" action, not "verify")
10. Address form filling (with user prompts)
11. Payment method selection
12. Final verification before order placement

Available actions:
- navigate: Go to URL (target: URL)
- click: Click element (target: CSS selector)
- type: Type text (target: selector, value: text, parameters: {submit: "true/false"})
- wait: Wait for element or duration (target: selector optional, value: duration)
- scroll: Scroll page (parameters: {direction: "up/down/top/bottom", amount: "500"})
- go_back: Navigate back to previous page
- select_product: Intelligently select product (value: criteria like "first", "rating above 4", "cheapest", "highest rated")
- add_to_cart: Add current product to cart
- proceed_checkout: Navigate to checkout from cart
- login: Handle authentication - prompts user for email/password and fills them (parameters: {type: "full"})
- fill_address: Fill shipping address form (prompts user)
- select_payment: Select payment method
- extract: Extract text (target: selector)
- verify: Verify page state (target: selector optional, value: expected text)

CRITICAL AUTHENTICATION GUIDELINES:
1. After "proceed_checkout", expect a signin/login page
2. Use "login" action (NOT "verify" with #ap_email selector)
3. The login action will:
   - Prompt user for email/password in terminal
   - Fill the credentials into the form
   - Submit the login form
4. After login action, add a wait step for page to load

CRITICAL SELECTOR GUIDELINES:
1. Use SPECIFIC Amazon selectors like #twotabsearchtextbox, #add-to-cart-button
2. For product pages, do NOT use wait steps with #productTitle - it's unreliable
3. Instead, after select_product, just use: wait with value "4s" (no target selector)
4. The select_product action handles navigation and verification internally
5. After product selection, the page URL will contain /dp/ which confirms we're on product page

Important guidelines:
1. Include wait steps after navigation (2-3 seconds)
2. Add scrolling steps to explore products
3. Use select_product for intelligent product selection
4. Include verification steps after critical actions
5. For login, use the "login" action with parameters: {"type": "full"}
6. Break down address filling into logical steps
7. Add error recovery checkpoints
8. Make the plan detailed enough to reach checkout screen (30-50 steps minimum)
9. NEVER use #productTitle in wait steps - use duration-only waits after product selection
10. ALWAYS provide valid CSS selectors for click/type actions

Return ONLY valid JSON in this format:
{
  "steps": [
    {
      "action": "navigate",
      "description": "Navigate to Amazon homepage",
      "target": "https://www.amazon.in",
      "critical": true
    },
    {
      "action": "wait",
      "description": "Wait for page to load",
      "value": "3s",
      "critical": false
    },
    {
      "action": "proceed_checkout",
      "description": "Click proceed to checkout",
      "critical": true
    },
    {
      "action": "wait",
      "description": "Wait for signin page",
      "value": "3s",
      "critical": false
    },
    {
      "action": "login",
      "description": "Enter login credentials",
      "parameters": {"type": "full"},
      "critical": true
    },
    {
      "action": "wait",
      "description": "Wait after login",
      "value": "3s",
      "critical": false
    }
  ]
}`, taskDescription)

	response, err := p.llm.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("generate plan: %w", err)
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

	var plan Plan
	if err := json.Unmarshal([]byte(response), &plan); err != nil {
		return nil, fmt.Errorf("parse plan: %w (response: %s)", err, response)
	}

	if len(plan.Steps) == 0 {
		return nil, fmt.Errorf("plan has no steps")
	}

	return &plan, nil
}

func (p *Planner) Replan(ctx *ExecutionContext, reason string) (*Plan, error) {
	executedStepsDesc := ""
	for i, step := range ctx.ExecutedSteps {
		status := "✓"
		if !step.Success {
			status = "✗"
		}
		executedStepsDesc += fmt.Sprintf("%d. %s %s\n", i+1, status, step.Step.Description)
	}

	memoryInfo := ""
	if ctx.Memory != nil {
		memoryInfo = fmt.Sprintf(`
Memory state:
- Products found: %d
- Selected product: %s
- Cart items: %d
- Current page: %s
`, len(ctx.Memory.ProductURLs), ctx.Memory.SelectedProduct, len(ctx.Memory.CartItems), ctx.Memory.CurrentPage)
	}

	prompt := fmt.Sprintf(`You are a browser automation planner. The current plan needs adjustment.

Original Task: %s

Steps executed so far:
%s

%s

Reason for replanning: %s

Generate a NEW complete plan that:
1. Considers what has already been accomplished
2. Addresses the issue that caused replanning
3. Continues from current state to complete the task
4. Maintains the same level of detail (20-40 steps)
5. Every step MUST have a valid action from: navigate, click, type, wait, verify, login, scroll, select_product, add_to_cart, proceed_checkout, fill_address, select_payment, extract

Return ONLY valid JSON in the same format as before.`, ctx.TaskDescription, executedStepsDesc, memoryInfo, reason)

	response, err := p.llm.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("generate replan: %w", err)
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

	var plan Plan
	if err := json.Unmarshal([]byte(response), &plan); err != nil {
		return nil, fmt.Errorf("parse replan: %w", err)
	}

	if len(plan.Steps) == 0 {
		return nil, fmt.Errorf("replan produced no steps")
	}

	for i, step := range plan.Steps {
		if step.Action == "" {
			return nil, fmt.Errorf("step %d has empty action", i+1)
		}

		switch step.Action {
		case "click", "type", "extract", "verify":
			if step.Target == "" {
				return nil, fmt.Errorf("step %d (action: %s) must have a target selector", i+1, step.Action)
			}
		case "navigate":
			if step.Target == "" {
				return nil, fmt.Errorf("step %d (navigate) must have a URL", i+1)
			}
		}
	}

	return &plan, nil
}

func (p *Planner) CreateRecoveryPlan(ctx *ExecutionContext, pageState *browser.PageState, errorMsg string) (*Plan, error) {
	executedStepsDesc := ""
	for i, step := range ctx.ExecutedSteps[max(0, len(ctx.ExecutedSteps)-5):] {
		status := "✓"
		if !step.Success {
			status = "✗"
		}
		executedStepsDesc += fmt.Sprintf("%d. %s %s\n", i+1, status, step.Step.Description)
	}

	contentPreview := pageState.Content
	if len(contentPreview) > 1000 {
		contentPreview = contentPreview[:1000] + "..."
	}

	prompt := fmt.Sprintf(`You are a browser automation recovery planner. The agent encountered multiple consecutive failures.

Original Task: %s

Recent steps (last 5):
%s

Error: %s

Current Page State:
- URL: %s
- Title: %s
- Content: %s

Create a recovery plan that:
1. Diagnoses what went wrong
2. Takes corrective action (reload page, go back, try alternative approach)
3. Resumes the original task from a stable state
4. Uses 10-20 steps to recover and continue

Return ONLY valid JSON with recovery steps.`, ctx.TaskDescription, executedStepsDesc, errorMsg, pageState.URL, pageState.Title, contentPreview)

	response, err := p.llm.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("generate recovery plan: %w", err)
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

	var plan Plan
	if err := json.Unmarshal([]byte(response), &plan); err != nil {
		return nil, fmt.Errorf("parse recovery plan: %w", err)
	}

	return &plan, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}