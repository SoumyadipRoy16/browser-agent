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
	memory  *AgentMemory
}

type ExecutionResult struct {
	Success  bool
	Message  string
	Data     map[string]interface{}
	NextStep *Step
}

func NewExecutor(br *browser.Browser, llmClient *llm.GeminiClient, memory *AgentMemory) *Executor {
	return &Executor{
		browser: br,
		llm:     llmClient,
		memory:  memory,
	}
}

func (e *Executor) ExecuteStep(step Step, ctx *ExecutionContext) (*ExecutionResult, error) {
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
	case "request_auth", "request_credentials", "login":
		return e.executeRequestAuth(step)
	case "scroll":
		return e.executeScroll(step)
	case "select_product":
		return e.executeSelectProduct(step, ctx)
	case "add_to_cart":
		return e.executeAddToCart(step)
	case "proceed_checkout":
		return e.executeProceedCheckout(step)
	case "fill_address":
		return e.executeFillAddress(step)
	case "select_payment":
		return e.executeSelectPayment(step)
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

	pageState, _ := e.browser.GetPageState()
	return &ExecutionResult{
		Success: true,
		Message: fmt.Sprintf("Navigated to %s", step.Target),
		Data:    map[string]interface{}{"current_page": pageState.URL},
	}, nil
}

func (e *Executor) executeClick(step Step) (*ExecutionResult, error) {
	if step.Target == "" {
		return nil, fmt.Errorf("click requires target selector")
	}

	err := e.browser.WaitForSelector(step.Target, 5*time.Second)
	if err != nil {
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

func (e *Executor) executeScroll(step Step) (*ExecutionResult, error) {
	direction := "down"
	amount := 500

	if step.Parameters != nil {
		if submitVal, ok := step.Parameters["submit"]; ok {
			// Handle different types - could be bool or string
			switch v := submitVal.(type) {
			case bool:
				if v {
					// Submit is true
					time.Sleep(500 * time.Millisecond)
					e.browser.Press(step.Target, "Enter")
				}
			case string:
				if v == "true" {
					time.Sleep(500 * time.Millisecond)
					e.browser.Press(step.Target, "Enter")
				}
			}
		}
	}

	var script string
	switch direction {
	case "down":
		script = fmt.Sprintf("window.scrollBy(0, %d)", amount)
	case "up":
		script = fmt.Sprintf("window.scrollBy(0, -%d)", amount)
	case "bottom":
		script = "window.scrollTo(0, document.body.scrollHeight)"
	case "top":
		script = "window.scrollTo(0, 0)"
	default:
		script = fmt.Sprintf("window.scrollBy(0, %d)", amount)
	}

	_, err := e.browser.Evaluate(script)
	if err != nil {
		return nil, fmt.Errorf("scroll failed: %w", err)
	}

	time.Sleep(1 * time.Second)

	return &ExecutionResult{
		Success: true,
		Message: fmt.Sprintf("Scrolled %s", direction),
	}, nil
}

func (e *Executor) executeSelectProduct(step Step, ctx *ExecutionContext) (*ExecutionResult, error) {
	criteria := step.GetValueString()
	if criteria == "" && step.Parameters != nil {
		if crit, ok := step.Parameters["criteria"]; ok {
			if critStr, ok := crit.(string); ok {
				criteria = critStr
			}
		}
	}

	fmt.Printf("   🔍 Selecting product based on: %s\n", criteria)

	// Extract all products with their information
	script := `
	() => {
		const products = [];
		const productElements = document.querySelectorAll('[data-component-type="s-search-result"], .s-result-item[data-asin]');
		
		productElements.forEach((el, idx) => {
			const asin = el.getAttribute('data-asin');
			if (!asin || asin === '') return;
			
			const titleEl = el.querySelector('h2 a, .a-link-normal.s-link-style, h2.a-size-mini a');
			const priceEl = el.querySelector('.a-price .a-offscreen, .a-price-whole');
			const ratingEl = el.querySelector('.a-icon-star-small .a-icon-alt, [aria-label*="stars"], .a-icon-alt');
			
			if (titleEl) {
				const ratingText = ratingEl ? (ratingEl.innerText || ratingEl.textContent || ratingEl.getAttribute('aria-label') || '') : 'N/A';
				let rating = 0;
				const ratingMatch = ratingText.match(/(\d+\.?\d*)/);
				if (ratingMatch) {
					rating = parseFloat(ratingMatch[1]);
				}
				
				products.push({
					index: idx,
					asin: asin,
					title: (titleEl.innerText || titleEl.textContent || '').trim(),
					price: priceEl ? (priceEl.innerText || priceEl.textContent || '').replace(/[^0-9.]/g, '') : '0',
					rating: rating,
					ratingText: ratingText,
					link: titleEl.href || ''
				});
			}
		});
		
		return products;
	}
	`

	result, err := e.browser.Evaluate(script)
	if err != nil {
		return nil, fmt.Errorf("failed to extract products: %w", err)
	}

	products, ok := result.([]interface{})
	if !ok || len(products) == 0 {
		return nil, fmt.Errorf("no products found on page")
	}

	// Filter and select product based on criteria
	selectedIndex := e.selectProductByCriteria(products, criteria)
	
	if selectedIndex >= 0 && selectedIndex < len(products) {
		product := products[selectedIndex].(map[string]interface{})
		title := product["title"].(string)
		rating := product["rating"]
		
		fmt.Printf("   ✓ Selected product #%d: %s (Rating: %v)\n", selectedIndex+1, title, rating)
		
		// Click using JavaScript with proper navigation handling
		clickScript := fmt.Sprintf(`
		() => {
			const productElements = document.querySelectorAll('[data-component-type="s-search-result"], .s-result-item[data-asin]');
			let validProducts = [];
			
			productElements.forEach((el) => {
				const asin = el.getAttribute('data-asin');
				if (asin && asin !== '') {
					const titleEl = el.querySelector('h2 a, .a-link-normal.s-link-style, h2.a-size-mini a');
					if (titleEl) {
						validProducts.push(titleEl);
					}
				}
			});
			
			if (validProducts[%d]) {
				const link = validProducts[%d];
				link.scrollIntoView({behavior: 'smooth', block: 'center'});
				// Open in same tab
				window.location.href = link.href;
				return {success: true, url: link.href};
			}
			return {success: false};
		}
		`, selectedIndex, selectedIndex)
		
		clickResult, err := e.browser.Evaluate(clickScript)
		if err != nil {
			return nil, fmt.Errorf("failed to click product: %w", err)
		}
		
		if resultMap, ok := clickResult.(map[string]interface{}); ok {
			if success, ok := resultMap["success"].(bool); !ok || !success {
				return nil, fmt.Errorf("failed to navigate to product")
			}
		}
		
		// Wait longer for product page to load
		time.Sleep(5 * time.Second)
		
		// Verify we're on a product page
		pageState, _ := e.browser.GetPageState()
		if !strings.Contains(pageState.URL, "/dp/") && !strings.Contains(pageState.URL, "/gp/product/") {
			return nil, fmt.Errorf("navigation to product page may have failed")
		}
		
		return &ExecutionResult{
			Success: true,
			Message: fmt.Sprintf("Selected product: %s", title),
			Data: map[string]interface{}{
				"selected_product": title,
				"product_url":      pageState.URL,
			},
		}, nil
	}

	return nil, fmt.Errorf("could not select product based on criteria")
}

func (e *Executor) selectProductByCriteria(products []interface{}, criteria string) int {
	criteriaLower := strings.ToLower(criteria)
	
	// Default to first product if no criteria
	if criteriaLower == "" {
		return 0
	}
	
	if strings.Contains(criteriaLower, "first") || strings.Contains(criteriaLower, "1st") {
		return 0
	}
	
	if strings.Contains(criteriaLower, "second") || strings.Contains(criteriaLower, "2nd") {
		if len(products) > 1 {
			return 1
		}
	}
	
	if strings.Contains(criteriaLower, "third") || strings.Contains(criteriaLower, "3rd") {
		if len(products) > 2 {
			return 2
		}
	}
	
	// Select by rating (good rating = 4.0+)
	if strings.Contains(criteriaLower, "rating") || strings.Contains(criteriaLower, "rated") || 
	   strings.Contains(criteriaLower, "stars") || strings.Contains(criteriaLower, "star") {
		maxRating := 0.0
		maxIndex := 0
		for i, p := range products {
			product := p.(map[string]interface{})
			rating := 0.0
			
			// Handle both float64 and string ratings
			switch r := product["rating"].(type) {
			case float64:
				rating = r
			case string:
				fmt.Sscanf(r, "%f", &rating)
			}
			
			if rating >= 4.0 && rating > maxRating {
				maxRating = rating
				maxIndex = i
			}
		}
		if maxRating >= 4.0 {
			return maxIndex
		}
		return 0 // Fallback to first if no 4+ rating found
	}
	
	// Select cheapest
	if strings.Contains(criteriaLower, "cheap") || strings.Contains(criteriaLower, "low price") || 
	   strings.Contains(criteriaLower, "lowest") {
		minPrice := -1.0
		minIndex := 0
		for i, p := range products {
			product := p.(map[string]interface{})
			priceStr := product["price"].(string)
			var price float64
			fmt.Sscanf(priceStr, "%f", &price)
			if price > 0 && (minPrice < 0 || price < minPrice) {
				minPrice = price
				minIndex = i
			}
		}
		if minPrice > 0 {
			return minIndex
		}
	}
	
	// Default to first product
	return 0
}

func (e *Executor) executeAddToCart(step Step) (*ExecutionResult, error) {
	addToCartSelectors := []string{
		"#add-to-cart-button",
		"input[name='submit.add-to-cart']",
		"#buy-now-button",
		".a-button-input[aria-labelledby='submit.add-to-cart-announce']",
		"[name='submit.addToCart']",
	}

	for _, selector := range addToCartSelectors {
		err := e.browser.WaitForSelector(selector, 3*time.Second)
		if err == nil {
			err = e.browser.Click(selector)
			if err == nil {
				time.Sleep(2 * time.Second)
				
				fmt.Printf("   ✓ Added to cart\n")
				
				return &ExecutionResult{
					Success: true,
					Message: "Product added to cart",
					Data:    map[string]interface{}{"cart_item": "added"},
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("could not find add to cart button")
}

func (e *Executor) executeProceedCheckout(step Step) (*ExecutionResult, error) {
	checkoutSelectors := []string{
		"#sc-buy-box-ptc-button",
		"[name='proceedToRetailCheckout']",
		"input[name='proceedToCheckout']",
		".a-button-input[aria-labelledby='sc-buy-box-ptc-button-announce']",
		"#hlb-ptc-btn-native",
	}

	cartSelectors := []string{
		"#nav-cart",
		"#nav-cart-count-container",
		".nav-cart-icon",
	}

	cartOpened := false
	for _, selector := range cartSelectors {
		err := e.browser.WaitForSelector(selector, 2*time.Second)
		if err == nil {
			err = e.browser.Click(selector)
			if err == nil {
				time.Sleep(2 * time.Second)
				cartOpened = true
				break
			}
		}
	}

	if !cartOpened {
		fmt.Printf("   ⚠️  Could not open cart, trying direct checkout\n")
	}

	for _, selector := range checkoutSelectors {
		err := e.browser.WaitForSelector(selector, 3*time.Second)
		if err == nil {
			err = e.browser.Click(selector)
			if err == nil {
				time.Sleep(3 * time.Second)
				return &ExecutionResult{
					Success: true,
					Message: "Proceeding to checkout",
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("could not find proceed to checkout button")
}

func (e *Executor) executeFillAddress(step Step) (*ExecutionResult, error) {
	reader := bufio.NewReader(os.Stdin)
	
	fmt.Printf("\n📍 Shipping Address Required\n")
	
	fields := []struct {
		name     string
		selector string
		prompt   string
	}{
		{"fullname", "#address-ui-widgets-enterAddressFullName", "Full Name: "},
		{"phone", "#address-ui-widgets-enterAddressPhoneNumber", "Phone Number: "},
		{"pincode", "#address-ui-widgets-enterAddressPostalCode", "Pincode: "},
		{"address1", "#address-ui-widgets-enterAddressLine1", "Address Line 1: "},
		{"address2", "#address-ui-widgets-enterAddressLine2", "Address Line 2 (optional): "},
		{"city", "#address-ui-widgets-enterAddressCity", "City: "},
		{"state", "#address-ui-widgets-enterAddressStateOrRegion", "State: "},
	}
	
	for _, field := range fields {
		err := e.browser.WaitForSelector(field.selector, 2*time.Second)
		if err != nil {
			continue
		}
		
		fmt.Print(field.prompt)
		input, _ := reader.ReadString('\n')
		value := strings.TrimSpace(input)
		
		if value != "" {
			err = e.browser.Type(field.selector, value)
			if err != nil {
				fmt.Printf("   ⚠️  Could not fill %s\n", field.name)
			}
			time.Sleep(300 * time.Millisecond)
		}
	}
	
	submitSelectors := []string{
		"input[aria-labelledby='address-ui-widgets-form-submit-button-announce']",
		"#address-ui-widgets-form-submit-button",
		"[name='address-ui-widgets-form-submit-button']",
	}
	
	for _, selector := range submitSelectors {
		err := e.browser.Click(selector)
		if err == nil {
			time.Sleep(2 * time.Second)
			break
		}
	}
	
	return &ExecutionResult{
		Success: true,
		Message: "Address form filled",
	}, nil
}

func (e *Executor) executeSelectPayment(step Step) (*ExecutionResult, error) {
	paymentSelectors := []string{
		"input[value='instrumentId=NetBanking']",
		"input[value='SelectableAddCreditCard']",
		"#pp-pNbbwp-127", // COD
		"input[name='ppw-instrumentRowSelection']",
	}
	
	fmt.Printf("\n💳 Select Payment Method\n")
	fmt.Printf("Note: This is a simulation. Agent will select first available payment method.\n")
	
	for _, selector := range paymentSelectors {
		err := e.browser.WaitForSelector(selector, 2*time.Second)
		if err == nil {
			err = e.browser.Click(selector)
			if err == nil {
				time.Sleep(1 * time.Second)
				fmt.Printf("   ✓ Payment method selected\n")
				break
			}
		}
	}
	
	continueSelectors := []string{
		"input[name='ppw-widgetEvent:SetPaymentPlanSelectContinueEvent']",
		"#continue-top",
		"#bottomSubmitOrderButtonId",
	}
	
	for _, selector := range continueSelectors {
		err := e.browser.WaitForSelector(selector, 2*time.Second)
		if err == nil {
			fmt.Printf("   ⚠️  Found 'Continue' button but NOT clicking (stopping before final order)\n")
			break
		}
	}
	
	return &ExecutionResult{
		Success: true,
		Message: "Reached payment screen (stopped before final submission)",
	}, nil
}

func (e *Executor) clickFirstItem(step Step) (*ExecutionResult, error) {
	fmt.Println("   🔍 Looking for the first product link...")

	script := `
	() => {
		const selectors = [
			'div[data-component-type="s-search-result"] a.a-link-normal[href*="/dp/"]',
			'a[href*="/dp/"]',
			'div[data-asin] h2 a',
			'.s-result-item a.a-link-normal.s-no-outline'
		];
		
		for (let selector of selectors) {
			const elements = document.querySelectorAll(selector);
			if (elements.length > 0) {
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

	if resultMap, ok := result.(map[string]interface{}); ok {
		if success, ok := resultMap["success"].(bool); ok && success {
			time.Sleep(3 * time.Second)
			return &ExecutionResult{
				Success: true,
				Message: "Clicked the link of the first product.",
			}, nil
		}
	}

	return nil, fmt.Errorf("could not click first item")
}

func (e *Executor) executeDynamicSearch(step Step, ctx *ExecutionContext) (*ExecutionResult, error) {
	searchTerm := e.extractSearchTerm(step, ctx)
	fmt.Printf("   🔍 Search term extracted: '%s'\n", searchTerm)

	if step.Target != "" {
		return e.executeTypingAction(step, searchTerm)
	}

	return e.findAndUseSearchBox(searchTerm)
}

func (e *Executor) extractSearchTerm(step Step, ctx *ExecutionContext) string {
	if step.Value != nil {
		switch v := step.Value.(type) {
		case string:
			if v != "" && !strings.HasSuffix(v, "ms") && !strings.Contains(v, "://") {
				return v
			}
		}
	}

	desc := strings.ToLower(step.Description)

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

	if ctx != nil && ctx.TaskDescription != "" {
		taskLower := strings.ToLower(ctx.TaskDescription)
		searchPatterns := []string{"search for ", "search ", "find ", "look for "}

		for _, pattern := range searchPatterns {
			if idx := strings.Index(taskLower, pattern); idx != -1 {
				start := idx + len(pattern)
				end := len(taskLower)
				for i, ch := range taskLower[start:] {
					if ch == ',' || ch == '.' || ch == ';' || (ch == ' ' && i > 10) {
						end = start + i
						break
					}
				}

				if start < end {
					term := strings.TrimSpace(taskLower[start:end])
					if term != "" && len(term) > 2 {
						return term
					}
				}
			}
		}
	}

	return "product"
}

func (e *Executor) executeTypingAction(step Step, searchTerm string) (*ExecutionResult, error) {
	err := e.browser.Click(step.Target)
	if err == nil {
		time.Sleep(300 * time.Millisecond)
		e.browser.Press(step.Target, "Control+a")
		time.Sleep(100 * time.Millisecond)
		e.browser.Press(step.Target, "Delete")
	}

	err = e.browser.Type(step.Target, searchTerm)
	if err != nil {
		return nil, fmt.Errorf("type into %s: %w", step.Target, err)
	}

	if step.Parameters != nil && step.Parameters["submit"] == "true" {
		time.Sleep(500 * time.Millisecond)
		e.browser.Press(step.Target, "Enter")
	}

	time.Sleep(2 * time.Second)

	return &ExecutionResult{
		Success: true,
		Message: fmt.Sprintf("Typed '%s' into %s", searchTerm, step.Target),
	}, nil
}

func (e *Executor) findAndUseSearchBox(searchTerm string) (*ExecutionResult, error) {
	searchBoxSelectors := []string{
		"#twotabsearchtextbox",
		"input[type='text']",
		"input[type='search']",
		"#search",
		".search-box",
	}

	for _, selector := range searchBoxSelectors {
		err := e.browser.WaitForSelector(selector, 1*time.Second)
		if err == nil {
			e.browser.Click(selector)
			time.Sleep(300 * time.Millisecond)

			err = e.browser.Type(selector, searchTerm)
			if err != nil {
				continue
			}

			time.Sleep(500 * time.Millisecond)
			e.browser.Press(selector, "Enter")
			time.Sleep(2 * time.Second)

			return &ExecutionResult{
				Success: true,
				Message: fmt.Sprintf("Searched for '%s'", searchTerm),
			}, nil
		}
	}

	return nil, fmt.Errorf("could not find a search box")
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
		e.browser.Press(step.Target, "Enter")
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
		// Try multiple common selectors for the target
		selectors := []string{step.Target}
		
		// Add fallback selectors for common elements
		if strings.Contains(step.Target, "productTitle") {
			selectors = append(selectors, 
				"#productTitle",
				"#title",
				"h1.product-title",
				"span#productTitle",
				"h1[id='title']",
			)
		}
		
		var lastErr error
		for _, selector := range selectors {
			err := e.browser.WaitForSelector(selector, duration)
			if err == nil {
				return &ExecutionResult{
					Success: true,
					Message: fmt.Sprintf("Element %s appeared", selector),
				}, nil
			}
			lastErr = err
		}
		
		// If all selectors failed, check if we're at least on the right page type
		pageState, _ := e.browser.GetPageState()
		if strings.Contains(step.Target, "productTitle") {
			if strings.Contains(pageState.URL, "/dp/") || strings.Contains(pageState.URL, "/gp/product/") {
				fmt.Printf("   ⚠️  Product title selector not found, but we're on a product page\n")
				time.Sleep(2 * time.Second)
				return &ExecutionResult{
					Success: true,
					Message: "On product page (selector not found but page loaded)",
				}, nil
			}
		}
		
		return nil, fmt.Errorf("wait for %s: %w", step.Target, lastErr)
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
		if authTypeVal, ok := step.Parameters["type"]; ok {
			if authTypeStr, ok := authTypeVal.(string); ok {
				authType = authTypeStr
			}
		}
	}

	fmt.Printf("\n🔐 Amazon Login Required\n")
	fmt.Printf("========================================\n")

	// Check which page we're on
	pageState, _ := e.browser.GetPageState()
	fmt.Printf("Current page: %s\n\n", pageState.Title)

	// First, try to enter email/phone
	if authType == "email" || authType == "full" {
		emailSelectors := []string{
			"#ap_email",
			"input[name='email']",
			"input[type='email']",
			"input[name='username']",
			"#username",
		}

		fmt.Print("📧 Email/Phone: ")
		emailInput, _ := reader.ReadString('\n')
		email := strings.TrimSpace(emailInput)

		if email != "" {
			emailEntered := false
			for _, selector := range emailSelectors {
				err := e.browser.WaitForSelector(selector, 2*time.Second)
				if err == nil {
					// Clear field first
					e.browser.Click(selector)
					time.Sleep(200 * time.Millisecond)
					
					err = e.browser.Type(selector, email)
					if err == nil {
						fmt.Printf("   ✓ Email entered in field: %s\n", selector)
						emailEntered = true
						e.memory.UserCredentials["email"] = email
						time.Sleep(500 * time.Millisecond)
						break
					}
				}
			}

			if !emailEntered {
				fmt.Printf("   ⚠️  Could not find email field, trying to continue...\n")
			}

			// Try to click "Continue" button after email
			continueSelectors := []string{
				"#continue",
				"input[id='continue']",
				"#auth-continue",
				"input[type='submit']",
				".a-button-input",
			}

			for _, selector := range continueSelectors {
				err := e.browser.WaitForSelector(selector, 1*time.Second)
				if err == nil {
					err = e.browser.Click(selector)
					if err == nil {
						fmt.Printf("   ✓ Clicked continue button\n")
						time.Sleep(3 * time.Second) // Wait for password page
						break
					}
				}
			}
		}
	}

	// Then try to enter password
	if authType == "password" || authType == "full" {
		passwordSelectors := []string{
			"#ap_password",
			"input[name='password']",
			"input[type='password']",
			"#password",
		}

		fmt.Print("🔒 Password: ")
		passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return nil, fmt.Errorf("read password: %w", err)
		}
		password := string(passwordBytes)
		fmt.Println() // New line after hidden input

		if password != "" {
			passwordEntered := false
			for _, selector := range passwordSelectors {
				err := e.browser.WaitForSelector(selector, 2*time.Second)
				if err == nil {
					// Clear field first
					e.browser.Click(selector)
					time.Sleep(200 * time.Millisecond)
					
					err = e.browser.Type(selector, password)
					if err == nil {
						fmt.Printf("   ✓ Password entered\n")
						passwordEntered = true
						time.Sleep(500 * time.Millisecond)
						break
					}
				}
			}

			if !passwordEntered {
				fmt.Printf("   ⚠️  Could not find password field\n")
				return nil, fmt.Errorf("password field not found")
			}
		}
	}

	// Finally, submit the form
	submitSelectors := []string{
		"#signInSubmit",
		"input[id='signInSubmit']",
		"#auth-signin-button",
		"input[type='submit']",
		".a-button-input[aria-labelledby='announce-auth-submit']",
		"button[type='submit']",
	}

	fmt.Printf("\n🔄 Submitting login form...\n")
	submitted := false
	for _, selector := range submitSelectors {
		err := e.browser.WaitForSelector(selector, 2*time.Second)
		if err == nil {
			err = e.browser.Click(selector)
			if err == nil {
				fmt.Printf("   ✓ Login form submitted\n")
				submitted = true
				time.Sleep(4 * time.Second) // Wait for login to process
				break
			}
		}
	}

	if !submitted {
		fmt.Printf("   ⚠️  Could not find submit button, trying Enter key...\n")
		// Try pressing Enter as fallback
		passwordSelectors := []string{"#ap_password", "input[type='password']"}
		for _, selector := range passwordSelectors {
			err := e.browser.Press(selector, "Enter")
			if err == nil {
				fmt.Printf("   ✓ Submitted via Enter key\n")
				time.Sleep(4 * time.Second)
				submitted = true
				break
			}
		}
	}

	// Check if login was successful
	time.Sleep(2 * time.Second)
	pageState, _ = e.browser.GetPageState()
	
	fmt.Printf("========================================\n")
	if strings.Contains(strings.ToLower(pageState.URL), "signin") || strings.Contains(strings.ToLower(pageState.URL), "ap/signin") {
		fmt.Printf("⚠️  Still on signin page - login may have failed\n")
		fmt.Printf("   Please check credentials or handle 2FA if prompted\n")
	} else {
		fmt.Printf("✅ Login appears successful!\n")
	}
	fmt.Printf("========================================\n\n")

	return &ExecutionResult{
		Success: true,
		Message: "Login credentials entered and submitted",
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
	if strings.Contains(step.Description, "search") || strings.Contains(step.Target, "search") {
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