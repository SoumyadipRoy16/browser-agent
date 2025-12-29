package amazon_agent

import (
	"fmt"
	"time"

	"browser-agent/internal/browser"
	"browser-agent/internal/config"
	"browser-agent/internal/llm"
)

type Agent struct {
	config    *config.Config
	browser   *browser.Browser
	planner   *Planner
	executor  *Executor
	validator *Validator
	memory    *AgentMemory
}

type AgentMemory struct {
	ProductURLs      []string
	SelectedProduct  string
	CartItems        []string
	CurrentPage      string
	UserCredentials  map[string]string
	SessionData      map[string]interface{}
}

type TaskResult struct {
	Success        bool
	StepsExecuted  int
	Duration       time.Duration
	FinalState     string
	Error          error
	Memory         *AgentMemory
}

func NewAgent(cfg *config.Config, apiKey string) (*Agent, error) {
	br, err := browser.NewBrowser(cfg.Headless, cfg.SlowMo)
	if err != nil {
		return nil, fmt.Errorf("create browser: %w", err)
	}

	llmClient := llm.NewGeminiClient(apiKey)

	memory := &AgentMemory{
		ProductURLs:     make([]string, 0),
		CartItems:       make([]string, 0),
		UserCredentials: make(map[string]string),
		SessionData:     make(map[string]interface{}),
	}

	return &Agent{
		config:    cfg,
		browser:   br,
		planner:   NewPlanner(llmClient),
		executor:  NewExecutor(br, llmClient, memory),
		validator: NewValidator(llmClient),
		memory:    memory,
	}, nil
}

func (a *Agent) ExecuteTask(taskDescription string) (*TaskResult, error) {
	startTime := time.Now()
	var lastValidationTime time.Time
	validationInterval := 5
	consecutiveFailures := 0
	maxConsecutiveFailures := 3

	plan, err := a.planner.CreatePlan(taskDescription)
	if err != nil {
		return nil, fmt.Errorf("create plan: %w", err)
	}

	fmt.Printf("📋 Generated plan with %d initial steps\n\n", len(plan.Steps))

	executionContext := &ExecutionContext{
		TaskDescription: taskDescription,
		Plan:            plan,
		ExecutedSteps:   []ExecutedStep{},
		CurrentStepNum:  0,
		Memory:          a.memory,
	}

	for executionContext.CurrentStepNum < len(plan.Steps) && executionContext.CurrentStepNum < a.config.MaxSteps {
		if time.Since(startTime) > a.config.TotalTimeout {
			return &TaskResult{
				Success:       false,
				StepsExecuted: len(executionContext.ExecutedSteps),
				Duration:      time.Since(startTime),
				Error:         fmt.Errorf("total timeout exceeded"),
				Memory:        a.memory,
			}, nil
		}

		step := plan.Steps[executionContext.CurrentStepNum]
		fmt.Printf("🔄 Step %d/%d: %s\n", executionContext.CurrentStepNum+1, len(plan.Steps), step.Description)

		executionResult, err := a.executor.ExecuteStep(step, executionContext)

		executedStep := ExecutedStep{
			Step:      step,
			Success:   err == nil,
			Error:     err,
			Timestamp: time.Now(),
		}
		executionContext.ExecutedSteps = append(executionContext.ExecutedSteps, executedStep)

		if err != nil {
			fmt.Printf("   ❌ Failed: %v\n", err)
			consecutiveFailures++

			if consecutiveFailures >= maxConsecutiveFailures {
				fmt.Printf("   🔄 Too many consecutive failures, attempting recovery...\n")
				pageState, _ := a.browser.GetPageState()
				recoveryPlan, recovErr := a.planner.CreateRecoveryPlan(executionContext, pageState, err.Error())
				if recovErr == nil && recoveryPlan != nil {
					plan = recoveryPlan
					executionContext.Plan = recoveryPlan
					executionContext.CurrentStepNum = 0
					consecutiveFailures = 0
					fmt.Printf("   📋 Recovery plan with %d steps\n", len(recoveryPlan.Steps))
					continue
				}
			}

			if step.Critical {
				fmt.Printf("   🔄 Retrying critical step...\n")
				time.Sleep(2 * time.Second)
				_, retryErr := a.executor.ExecuteStep(step, executionContext)
				if retryErr == nil {
					fmt.Printf("   ✓ Retry successful\n")
					err = nil
					executedStep.Success = true
					executedStep.Error = nil
					consecutiveFailures = 0
				} else {
					return &TaskResult{
						Success:       false,
						StepsExecuted: len(executionContext.ExecutedSteps),
						Duration:      time.Since(startTime),
						Error:         fmt.Errorf("critical step failed after retry: %w", retryErr),
						Memory:        a.memory,
					}, nil
				}
			}
		} else {
			fmt.Printf("   ✓ Completed\n")
			consecutiveFailures = 0

			// Store execution result in memory if available
			if executionResult != nil && executionResult.Data != nil {
				a.updateMemory(executionResult.Data)
			}
		}

		executionContext.CurrentStepNum++

		if err == nil && (executionContext.CurrentStepNum%validationInterval == 0) && time.Since(lastValidationTime) > 10*time.Second {
			pageState, _ := a.browser.GetPageState()
			validationResult, valErr := a.validator.ValidateProgress(executionContext, pageState)

			if valErr != nil {
				fmt.Printf("   ⚠️  Validation error: %v\n", valErr)
			} else if validationResult != nil {
				lastValidationTime = time.Now()
				if validationResult.IsComplete {
					fmt.Printf("\n🎉 Task completed: %s\n", validationResult.Message)
					return &TaskResult{
						Success:       true,
						StepsExecuted: len(executionContext.ExecutedSteps),
						Duration:      time.Since(startTime),
						FinalState:    validationResult.Message,
						Memory:        a.memory,
					}, nil
				}

				if validationResult.NeedsReplanning {
					fmt.Printf("   🔄 Replanning required: %s\n", validationResult.Message)
					newPlan, replanErr := a.planner.Replan(executionContext, validationResult.Message)
					if replanErr != nil {
						fmt.Printf("   ⚠️  Replan failed: %v, continuing with original plan\n", replanErr)
					} else {
						plan = newPlan
						executionContext.Plan = newPlan
						executionContext.CurrentStepNum = 0
						fmt.Printf("   📋 New plan with %d steps\n", len(newPlan.Steps))
						continue
					}
				}
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	if executionContext.CurrentStepNum >= a.config.MaxSteps {
		return &TaskResult{
			Success:       false,
			StepsExecuted: len(executionContext.ExecutedSteps),
			Duration:      time.Since(startTime),
			Error:         fmt.Errorf("max steps exceeded"),
			Memory:        a.memory,
		}, nil
	}

	return &TaskResult{
		Success:       true,
		StepsExecuted: len(executionContext.ExecutedSteps),
		Duration:      time.Since(startTime),
		FinalState:    "All planned steps completed",
		Memory:        a.memory,
	}, nil
}

func (a *Agent) updateMemory(data map[string]interface{}) {
	if url, ok := data["product_url"].(string); ok {
		a.memory.ProductURLs = append(a.memory.ProductURLs, url)
	}
	if selected, ok := data["selected_product"].(string); ok {
		a.memory.SelectedProduct = selected
	}
	if cartItem, ok := data["cart_item"].(string); ok {
		a.memory.CartItems = append(a.memory.CartItems, cartItem)
	}
	if page, ok := data["current_page"].(string); ok {
		a.memory.CurrentPage = page
	}
}

func (a *Agent) Close() {
	if a.browser != nil {
		a.browser.Close()
	}
}