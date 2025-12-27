package amazon_agent

import (
	"encoding/json"
	"fmt"
	"strings"

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
	Parameters  map[string]string `json:"parameters,omitempty"`
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
        // Determine if it's a wait time or something else
        // Wait times are usually small (milliseconds)
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
        // Try to convert to string
        return fmt.Sprintf("%v", v)
    }
}

type ExecutionContext struct {
	TaskDescription string
	Plan            *Plan
	ExecutedSteps   []ExecutedStep
	CurrentStepNum  int
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
    prompt := fmt.Sprintf(`You are a browser automation planner. Create a detailed step-by-step plan to accomplish this task:

Task: %s

Generate a JSON plan with steps. Each step should have:
- action: navigate, click, type, wait, extract, or verify
- description: what this step does
- target: CSS selector or URL (if applicable)
- value: text to type or data to extract (if applicable)
- critical: true if failure should stop execution

Important guidelines:
1. For search actions, use meaningful selectors like input[type="text"], #search, .search-box, etc.
2. When the task mentions a specific search term (like "detergent"), use that exact term in the value field
3. For clicking first items, use selectors like :first-child, :first-of-type, or [0] indices
4. Include wait steps after navigation or dynamic content loading
5. Add verification steps to check if actions succeeded
6. ALWAYS provide non-empty selectors for click, type, extract, and verify actions
7. NEVER use numbers as search terms - numbers should only be used for wait durations

Return ONLY valid JSON in this exact format:
{
  "steps": [
    {
      "action": "navigate",
      "description": "Go to homepage",
      "target": "https://example.com",
      "critical": true
    },
    {
      "action": "type",
      "description": "Enter search term",
      "target": "#search-box",
      "value": "laptop",
      "critical": true
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

	prompt := fmt.Sprintf(`You are a browser automation planner. The current plan needs adjustment.

Original Task: %s

Steps executed so far:
%s

Reason for replanning: %s

Generate a NEW complete plan that:
1. Considers what has already been accomplished
2. Addresses the issue that caused replanning
3. Continues from current state to complete the task
4. IMPORTANT: Every step MUST have a valid action from: navigate, click, type, wait, verify, request_auth, extract
5. Never use empty action strings or undefined actions

Return ONLY valid JSON in the same format as before:
{
  "steps": [...]
}`, ctx.TaskDescription, executedStepsDesc, reason)

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