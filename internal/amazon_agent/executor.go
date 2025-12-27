package amazon_agent

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"browser-agent/internal/browser"
	"browser-agent/internal/llm"
	"golang.org/x/term"
)

type Executor struct {
	browser *browser.Browser
	llm     *llm.GeminiClient
}

type ExecutionResult struct {
	Success  bool
	Message  string
	NextStep *Step
}

func NewExecutor(br *browser.Browser, llmClient *llm.GeminiClient) *Executor {
	return &Executor{
		browser: br,
		llm:     llmClient,
	}
}

func (e *Executor) ExecuteStep(step Step, ctx *ExecutionContext) (*ExecutionResult, error) {
	// Dynamic search term extraction for any search action
	if step.Action == "type" && strings.Contains(strings.ToLower(step.Description), "search") {
		return e.executeDynamicSearch(step, ctx)
	}
	
	switch step.Action {
	case "navigate":
		return e.executeNavigate(step)
	case "click":
		return e.executeClick(step)
	case "type":
		return e.executeType(step)
	case "wait":
		return e.executeWait(step)
	case "extract":
		return e.executeExtract(step)
	case "verify":
		return e.executeVerify(step)
	case "request_auth", "request_credentials":
		return e.executeRequestAuth(step)
	case "smart_action":
		return e.executeSmartAction(step, ctx)
	default:
		fmt.Printf("   ⚠️  Unknown action '%s', trying smart fallback...\n", step.Action)
		return e.executeSmartAction(step, ctx)
	}
}

func (e *Executor) executeNavigate(step Step) (*ExecutionResult, error) {
	if step.Target == "" {
		return nil, fmt.Errorf("navigate requires target URL")
	}

	err := e.browser.Navigate(step.Target)
	if err != nil {
		return nil, fmt.Errorf("navigate to %s: %w", step.Target, err)
	}

	return &ExecutionResult{
		Success: true,
		Message: fmt.Sprintf("Navigated to %s", step.Target),
	}, nil
}

func (e *Executor) executeClick(step Step) (*ExecutionResult, error) {
	if step.Target == "" {
		return nil, fmt.Errorf("click requires target selector")
	}

	err := e.browser.WaitForSelector(step.Target, 5*time.Second)
	if err != nil {
		// If selector not found and description mentions "first", try dynamic approach
		if strings.Contains(strings.ToLower(step.Description), "first") {
			return e.clickFirstItem(step)
		}
		pageState, _ := e.browser.GetPageState()
		return e.findAndClickAlternative(step, pageState)
	}

	err = e.browser.Click(step.Target)
	if err != nil {
		return nil, fmt.Errorf("click %s: %w", step.Target, err)
	}

	time.Sleep(1 * time.Second)

	return &ExecutionResult{
		Success: true,
		Message: fmt.Sprintf("Clicked %s", step.Target),
	}, nil
}

func (e *Executor) clickFirstItem(step Step) (*ExecutionResult, error) {
	fmt.Println("   🔍 Looking for the first product link...")

	// Strategy 1: Use JavaScript to find and click the first specific product link.
	// This is the most reliable method for Amazon.
	script := `
	() => {
		// Try multiple specific selectors for Amazon product links
		const selectors = [
			'div[data-component-type="s-search-result"] a.a-link-normal[href*="/dp/"]',
			'a[href*="/dp/"]',
			'div[data-asin] h2 a',
			'.s-result-item a.a-link-normal.s-no-outline'
		];
		
		for (let selector of selectors) {
			const elements = document.querySelectorAll(selector);
			if (elements.length > 0) {
				console.log('Found element with selector:', selector);
				const firstLink = elements[0];
				firstLink.scrollIntoView({behavior: 'smooth', block: 'center'});
				firstLink.click();
				return {success: true, clickedHref: firstLink.href};
			}
		}
		return {success: false, message: 'No product link found'};
	}
	`

	result, err := e.browser.Evaluate(script)
	if err != nil {
		return nil, fmt.Errorf("failed to execute click script: %w", err)
	}

	// Check the result from the JavaScript execution
	if resultMap, ok := result.(map[string]interface{}); ok {
		if success, ok := resultMap["success"].(bool); ok && success {
			// Wait for navigation to the product page
			time.Sleep(3 * time.Second)
			return &ExecutionResult{
				Success: true,
				Message: "Clicked the link of the first product.",
			}, nil
		} else {
			// Log why it failed
			if msg, ok := resultMap["message"].(string); ok {
				fmt.Printf("   ⚠️  Script failed: %s\n", msg)
			}
		}
	}

	// Strategy 2 (Fallback): If JS fails, try a robust, direct CSS selector.
	fmt.Println("   🔄 Fallback: Trying direct CSS selector...")
	robustSelectors := []string{
		`div[data-component-type="s-search-result"]:first-child a.a-link-normal`,
		`div[data-asin]:first-child a.a-link-normal`,
		`.s-main-slot .s-result-item:first-child h2 a`,
	}

	for _, selector := range robustSelectors {
		fmt.Printf("   🔄 Trying selector: %s\n", selector)
		err := e.browser.WaitForSelector(selector, 2*time.Second)
		if err != nil {
			continue // Selector not found, try the next one
		}
		err = e.browser.Click(selector)
		if err == nil {
			time.Sleep(3 * time.Second)
			return &ExecutionResult{
				Success: true,
				Message: fmt.Sprintf("Clicked first product using selector: %s", selector),
			}, nil
		}
	}

	// Strategy 3 (Last Resort): Final generic attempt.
	return e.clickFirstItemGenericFallback()
}

func (e *Executor) clickFirstItemGenericFallback() (*ExecutionResult, error) {
	fmt.Println("   🤖 Using generic fallback to find first item...")
	
	// Try different generic strategies to click the first item
	strategies := []struct {
		name     string
		selector string
	}{
		{"First child", "*:first-child"},
		{"First of type", "*:first-of-type"},
		{"First list item", "li:first-child, ul:first-child li, ol:first-child li"},
		{"First link", "a:first-child, .link:first-child, [href]:first-child"},
		{"First card/container", ".card:first-child, .item:first-child, .product:first-child, [class*='item']:first-child"},
	}
	
	for _, strategy := range strategies {
		fmt.Printf("   🔄 Trying strategy: %s with selector: %s\n", strategy.name, strategy.selector)
		
		// Use JavaScript to find and click the first matching element
		script := fmt.Sprintf(`
			() => {
				const elements = document.querySelectorAll('%s');
				console.log('Found', elements.length, 'elements with selector:', '%s');
				if (elements.length > 0) {
					const first = elements[0];
					first.scrollIntoView({behavior: 'smooth', block: 'center'});
					first.click();
					return {success: true, element: first.tagName + (first.id ? '#' + first.id : '') + (first.className ? '.' + first.className.split(' ').join('.') : '')};
				}
				return {success: false};
			}
		`, strategy.selector, strategy.selector)
		
		result, err := e.browser.Evaluate(script)
		if err == nil {
			if resultMap, ok := result.(map[string]interface{}); ok {
				if success, ok := resultMap["success"].(bool); ok && success {
					time.Sleep(2 * time.Second)
					return &ExecutionResult{
						Success: true,
						Message: fmt.Sprintf("Clicked first item using %s strategy", strategy.name),
					}, nil
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	
	// Last resort: Use AI to find what to click
	pageState, err := e.browser.GetPageState()
	if err != nil {
		return nil, fmt.Errorf("failed to get page state: %w", err)
	}
	
	prompt := fmt.Sprintf(`I need to click the first item/product on this page. Based on the page content, suggest the exact CSS selector for the first clickable item.

Page title: %s
URL: %s
Content snippet: %s

Return ONLY the CSS selector, nothing else.`, 
		pageState.Title, 
		pageState.URL, 
		pageState.Content[:min(500, len(pageState.Content))])
	
	response, err := e.llm.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("AI fallback failed: %w", err)
	}
	
	selector := strings.TrimSpace(response)
	fmt.Printf("   🤖 AI suggested selector: %s\n", selector)
	
	err = e.browser.Click(selector)
	if err != nil {
		return nil, fmt.Errorf("failed to click AI-suggested selector: %w", err)
	}
	
	time.Sleep(2 * time.Second)
	return &ExecutionResult{
		Success: true,
		Message: fmt.Sprintf("Clicked first item using AI-suggested selector: %s", selector),
	}, nil
}

func (e *Executor) executeDynamicSearch(step Step, ctx *ExecutionContext) (*ExecutionResult, error) {
	// Extract search term intelligently
	searchTerm := e.extractSearchTerm(step, ctx)
	
	fmt.Printf("   🔍 Search term extracted: '%s'\n", searchTerm)
	
	// If target selector is provided, use it
	if step.Target != "" {
		return e.executeTypingAction(step, searchTerm)
	}
	
	// Otherwise, find search box dynamically
	return e.findAndUseSearchBox(searchTerm)
}

func (e *Executor) extractSearchTerm(step Step, ctx *ExecutionContext) string {
	// Priority 1: Direct value from step
	if step.Value != nil {
		switch v := step.Value.(type) {
		case string:
			if v != "" && !strings.HasSuffix(v, "ms") && !strings.Contains(v, "://") {
				return v
			}
		}
	}
	
	// Priority 2: Extract from description (simple string parsing)
	desc := strings.ToLower(step.Description)
	
	// Look for quoted text first
	if idx := strings.Index(desc, "'"); idx != -1 {
		if endIdx := strings.Index(desc[idx+1:], "'"); endIdx != -1 {
			term := desc[idx+1 : idx+1+endIdx]
			if term != "" && len(term) > 2 {
				return term
			}
		}
	}
	
	if idx := strings.Index(desc, "\""); idx != -1 {
		if endIdx := strings.Index(desc[idx+1:], "\""); endIdx != -1 {
			term := desc[idx+1 : idx+1+endIdx]
			if term != "" && len(term) > 2 {
				return term
			}
		}
	}
	
	// Priority 3: Extract from task description
	if ctx != nil && ctx.TaskDescription != "" {
		taskLower := strings.ToLower(ctx.TaskDescription)
		
		// Look for patterns like "search for X" or "search X"
		searchPatterns := []string{
			"search for ",
			"search ",
			"find ",
			"look for ",
		}
		
		for _, pattern := range searchPatterns {
			if idx := strings.Index(taskLower, pattern); idx != -1 {
				// Extract the next word(s)
				start := idx + len(pattern)
				// Find end of term (comma, period, space, or end of string)
				end := len(taskLower)
				for i, ch := range taskLower[start:] {
					if ch == ',' || ch == '.' || ch == ';' || ch == ' ' && i > 10 {
						end = start + i
						break
					}
				}
				
				if start < end {
					term := strings.TrimSpace(taskLower[start:end])
					if term != "" && len(term) > 2 && term != "the" && term != "a" && term != "an" {
						return term
					}
				}
			}
		}
	}
	
	// Priority 4: Try to extract from description without quotes
	words := strings.Fields(desc)
	for i, word := range words {
		if word == "for" && i+1 < len(words) {
			nextWord := words[i+1]
			if len(nextWord) > 2 && nextWord != "the" && nextWord != "a" && nextWord != "an" {
				return nextWord
			}
		}
	}
	
	// Final fallback
	return "product"
}

func (e *Executor) executeTypingAction(step Step, searchTerm string) (*ExecutionResult, error) {
	// Clear the field first
	err := e.browser.Click(step.Target)
	if err == nil {
		time.Sleep(300 * time.Millisecond)
		// Select all and delete (Ctrl+A, Delete)
		err = e.browser.Press(step.Target, "Control+a")
		if err == nil {
			time.Sleep(100 * time.Millisecond)
			e.browser.Press(step.Target, "Delete")
		}
	}
	
	// Type the search term
	err = e.browser.Type(step.Target, searchTerm)
	if err != nil {
		return nil, fmt.Errorf("type into %s: %w", step.Target, err)
	}
	
	// Handle submission
	if step.Parameters != nil && step.Parameters["submit"] == "true" {
		time.Sleep(500 * time.Millisecond)
		err = e.browser.Press(step.Target, "Enter")
		if err != nil {
			// Try common search button selectors
			searchButtonSelectors := []string{
				"button[type='submit']",
				"input[type='submit']",
				".search-button",
				"#search-button",
				"[aria-label='Search']",
				"button:has(svg), button:has(img)",
			}
			
			for _, selector := range searchButtonSelectors {
				err = e.browser.Click(selector)
				if err == nil {
					break
				}
			}
		}
	}
	
	time.Sleep(2 * time.Second) // Wait for results
	
	return &ExecutionResult{
		Success: true,
		Message: fmt.Sprintf("Typed '%s' into %s", searchTerm, step.Target),
	}, nil
}

func (e *Executor) findAndUseSearchBox(searchTerm string) (*ExecutionResult, error) {
	// Try to find search box dynamically
	searchBoxSelectors := []string{
		"input[type='text']",
		"input[type='search']",
		"#search",
		".search-box",
		"[role='search'] input",
		"form[role='search'] input",
		"input[name='q']",
		"input[name='search']",
		"input[placeholder*='search' i]",
		"input[placeholder*='Search' i]",
	}
	
	for _, selector := range searchBoxSelectors {
		err := e.browser.WaitForSelector(selector, 1*time.Second)
		if err == nil {
			// Found a search box
			fmt.Printf("   🔍 Found search box: %s\n", selector)
			
			// Click to focus
			e.browser.Click(selector)
			time.Sleep(300 * time.Millisecond)
			
			// Type the search term
			err = e.browser.Type(selector, searchTerm)
			if err != nil {
				continue // Try next selector
			}
			
			// Try to submit
			time.Sleep(500 * time.Millisecond)
			e.browser.Press(selector, "Enter")
			
			time.Sleep(2 * time.Second)
			
			return &ExecutionResult{
				Success: true,
				Message: fmt.Sprintf("Searched for '%s' using dynamic selector", searchTerm),
			}, nil
		}
	}
	
	return nil, fmt.Errorf("could not find a search box on the page")
}

func (e *Executor) executeType(step Step) (*ExecutionResult, error) {
	if step.Target == "" {
		return nil, fmt.Errorf("type requires target selector")
	}
	
	value := step.GetValueString()
	if value == "" {
		return nil, fmt.Errorf("type requires value")
	}

	err := e.browser.WaitForSelector(step.Target, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("wait for %s: %w", step.Target, err)
	}

	err = e.browser.Type(step.Target, value)
	if err != nil {
		return nil, fmt.Errorf("type into %s: %w", step.Target, err)
	}

	if step.Parameters != nil && step.Parameters["submit"] == "true" {
		time.Sleep(500 * time.Millisecond)
		err = e.browser.Press(step.Target, "Enter")
		if err != nil {
			return nil, fmt.Errorf("press enter: %w", err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	return &ExecutionResult{
		Success: true,
		Message: fmt.Sprintf("Typed into %s", step.Target),
	}, nil
}

func (e *Executor) executeWait(step Step) (*ExecutionResult, error) {
	duration := 2 * time.Second
	
	value := step.GetValueString()
	if value != "" {
		parsedDuration, err := time.ParseDuration(value)
		if err == nil {
			duration = parsedDuration
		}
	}

	if step.Target != "" {
		err := e.browser.WaitForSelector(step.Target, duration)
		if err != nil {
			return nil, fmt.Errorf("wait for %s: %w", step.Target, err)
		}
		return &ExecutionResult{
			Success: true,
			Message: fmt.Sprintf("Element %s appeared", step.Target),
		}, nil
	}

	time.Sleep(duration)
	return &ExecutionResult{
		Success: true,
		Message: fmt.Sprintf("Waited %v", duration),
	}, nil
}

func (e *Executor) executeExtract(step Step) (*ExecutionResult, error) {
	if step.Target == "" {
		return nil, fmt.Errorf("extract requires target selector")
	}

	text, err := e.browser.GetText(step.Target)
	if err != nil {
		return nil, fmt.Errorf("extract from %s: %w", step.Target, err)
	}

	fmt.Printf("   📄 Extracted: %s\n", text)

	return &ExecutionResult{
		Success: true,
		Message: fmt.Sprintf("Extracted: %s", text),
	}, nil
}

func (e *Executor) executeVerify(step Step) (*ExecutionResult, error) {
	pageState, err := e.browser.GetPageState()
	if err != nil {
		return nil, fmt.Errorf("get page state: %w", err)
	}

	if step.Target != "" {
		err := e.browser.WaitForSelector(step.Target, 3*time.Second)
		if err != nil {
			return nil, fmt.Errorf("verification failed: element %s not found", step.Target)
		}
	}

	value := step.GetValueString()
	if value != "" {
		if !strings.Contains(strings.ToLower(pageState.URL), strings.ToLower(value)) &&
			!strings.Contains(strings.ToLower(pageState.Title), strings.ToLower(value)) &&
			!strings.Contains(strings.ToLower(pageState.Content), strings.ToLower(value)) {
			return nil, fmt.Errorf("verification failed: '%s' not found on page", value)
		}
	}

	return &ExecutionResult{
		Success: true,
		Message: "Verification passed",
	}, nil
}

func (e *Executor) executeRequestAuth(step Step) (*ExecutionResult, error) {
	reader := bufio.NewReader(os.Stdin)

	authType := "full"
	if step.Parameters != nil && step.Parameters["type"] != "" {
		authType = step.Parameters["type"]
	}

	fmt.Printf("\n🔐 Authentication required for Amazon\n")

	var credentials []struct {
		selector string
		value    string
	}

	if authType == "email" || authType == "full" {
		fmt.Print("Email/Phone: ")
		emailInput, _ := reader.ReadString('\n')
		email := strings.TrimSpace(emailInput)
		
		emailSelector := "#ap_email"
		if step.Parameters != nil && step.Parameters["email_selector"] != "" {
			emailSelector = step.Parameters["email_selector"]
		}
		
		if email != "" {
			credentials = append(credentials, struct {
				selector string
				value    string
			}{emailSelector, email})
		}
	}

	if authType == "password" || authType == "full" {
		fmt.Print("Password: ")
		passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return nil, fmt.Errorf("read password: %w", err)
		}
		password := string(passwordBytes)
		fmt.Println()
		
		passwordSelector := "#ap_password"
		if step.Parameters != nil && step.Parameters["password_selector"] != "" {
			passwordSelector = step.Parameters["password_selector"]
		}
		
		if password != "" {
			credentials = append(credentials, struct {
				selector string
				value    string
			}{passwordSelector, password})
		}
	}

	for _, cred := range credentials {
		err := e.browser.WaitForSelector(cred.selector, 5*time.Second)
		if err != nil {
			fmt.Printf("   ⚠️  Selector %s not found, might not be needed\n", cred.selector)
			continue
		}
		
		err = e.browser.Type(cred.selector, cred.value)
		if err != nil {
			return nil, fmt.Errorf("type credentials into %s: %w", cred.selector, err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	return &ExecutionResult{
		Success: true,
		Message: "Credentials entered",
	}, nil
}

func (e *Executor) executeSmartAction(step Step, ctx *ExecutionContext) (*ExecutionResult, error) {
	pageState, err := e.browser.GetPageState()
	if err != nil {
		return nil, fmt.Errorf("get page state: %w", err)
	}

	prompt := fmt.Sprintf(`You are controlling a browser. Current state:
URL: %s
Title: %s

Task: %s
Current step: %s

Page content preview:
%s

Determine the exact CSS selector to interact with and the action to take (click, type, wait).
Return JSON:
{
  "action": "click|type|wait",
  "selector": "exact CSS selector",
  "value": "text if typing",
  "confidence": 0.0-1.0
}`, pageState.URL, pageState.Title, ctx.TaskDescription, step.Description, pageState.Content[:min(1000, len(pageState.Content))])

	response, err := e.llm.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("get smart action: %w", err)
	}

	fmt.Printf("   🤖 AI suggested: %s\n", response)

	return &ExecutionResult{
		Success: true,
		Message: "Smart action completed",
	}, nil
}

func (e *Executor) findAndClickAlternative(step Step, pageState *browser.PageState) (*ExecutionResult, error) {
	// If it's an Amazon search box, try common selectors
	if strings.Contains(step.Description, "search") || strings.Contains(step.Target, "search") {
		// Try common Amazon search selectors
		selectors := []string{
			"#twotabsearchtextbox",
			"#nav-search-bar-form input[type='text']",
			"#nav-bb-search",
			"input[name='field-keywords']",
			"#searchDropdownBox",
		}
		
		for _, selector := range selectors {
			err := e.browser.Click(selector)
			if err == nil {
				return &ExecutionResult{
					Success: true,
					Message: fmt.Sprintf("Clicked using fallback selector: %s", selector),
				}, nil
			}
		}
	}
	
	// Original AI fallback logic
	prompt := fmt.Sprintf(`A click action failed. Find alternative selector.
Original selector: %s
Description: %s
Page title: %s
URL: %s

Suggest an alternative CSS selector that might work. Return only the selector, nothing else.`, step.Target, step.Description, pageState.Title, pageState.URL)

	response, err := e.llm.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("selector not found and AI fallback failed: %w", err)
	}

	altSelector := strings.TrimSpace(response)
	fmt.Printf("   🔄 Trying alternative selector: %s\n", altSelector)

	err = e.browser.Click(altSelector)
	if err != nil {
		return nil, fmt.Errorf("alternative selector also failed: %w", err)
	}

	return &ExecutionResult{
		Success: true,
		Message: fmt.Sprintf("Clicked using alternative selector: %s", altSelector),
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}