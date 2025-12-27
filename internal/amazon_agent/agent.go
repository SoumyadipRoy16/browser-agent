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
}

type TaskResult struct {
	Success        bool
	StepsExecuted  int
	Duration       time.Duration
	FinalState     string
	Error          error
}

func NewAgent(cfg *config.Config, apiKey string) (*Agent, error) {
	br, err := browser.NewBrowser(cfg.Headless, cfg.SlowMo)
	if err != nil {
		return nil, fmt.Errorf("create browser: %w", err)
	}

	llmClient := llm.NewGeminiClient(apiKey)

	return &Agent{
		config:    cfg,
		browser:   br,
		planner:   NewPlanner(llmClient),
		executor:  NewExecutor(br, llmClient),
		validator: NewValidator(llmClient),
	}, nil
}

func (a *Agent) ExecuteTask(taskDescription string) (*TaskResult, error) {
    startTime := time.Now()
    var lastValidationTime time.Time
    validationInterval := 3
    
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
    }

    for executionContext.CurrentStepNum < len(plan.Steps) && executionContext.CurrentStepNum < a.config.MaxSteps {
        if time.Since(startTime) > a.config.TotalTimeout {
            return &TaskResult{
                Success:       false,
                StepsExecuted: len(executionContext.ExecutedSteps),
                Duration:      time.Since(startTime),
                Error:         fmt.Errorf("total timeout exceeded"),
            }, nil
        }

        step := plan.Steps[executionContext.CurrentStepNum]
        fmt.Printf("🔄 Step %d/%d: %s\n", executionContext.CurrentStepNum+1, len(plan.Steps), step.Description)

        _, err := a.executor.ExecuteStep(step, executionContext)
        
        executedStep := ExecutedStep{
            Step:      step,
            Success:   err == nil,
            Error:     err,
            Timestamp: time.Now(),
        }
        executionContext.ExecutedSteps = append(executionContext.ExecutedSteps, executedStep)

        if err != nil {
            fmt.Printf("   ❌ Failed: %v\n", err)
            
            if step.Critical {
                fmt.Printf("   🔄 Retrying critical step...\n")
                time.Sleep(2 * time.Second)
                _, retryErr := a.executor.ExecuteStep(step, executionContext)
                if retryErr == nil {
                    fmt.Printf("   ✓ Retry successful\n")
                    err = nil
                    executedStep.Success = true
                    executedStep.Error = nil
                } else {
                    return &TaskResult{
                        Success:       false,
                        StepsExecuted: len(executionContext.ExecutedSteps),
                        Duration:      time.Since(startTime),
                        Error:         fmt.Errorf("critical step failed after retry: %w", retryErr),
                    }, nil
                }
            }
        } else {
            fmt.Printf("   ✓ Completed\n")
        }

        // **CRITICAL FIX: Always increment step number**
        // Move this increment BEFORE the validation check
        executionContext.CurrentStepNum++
        
        // **NEW: Only perform validation if step succeeded**
        if err == nil && (executionContext.CurrentStepNum % validationInterval == 0) && time.Since(lastValidationTime) > 10*time.Second {
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
                        executionContext.CurrentStepNum = 0 // Reset to start of new plan
                        fmt.Printf("   📋 New plan with %d steps\n", len(newPlan.Steps))
                        continue // Start executing new plan
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
        }, nil
    }

    return &TaskResult{
        Success:       true,
        StepsExecuted: len(executionContext.ExecutedSteps),
        Duration:      time.Since(startTime),
        FinalState:    "All planned steps completed",
    }, nil
}

func (a *Agent) Close() {
	if a.browser != nil {
		a.browser.Close()
	}
}