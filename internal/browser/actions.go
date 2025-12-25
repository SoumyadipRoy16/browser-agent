package browser

import (
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

// Click performs a click action on the specified selector
func (c *Controller) Click(selector string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := c.newContext(15 * time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Click(selector, chromedp.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("click failed on selector '%s': %w", selector, err)
	}

	return nil
}

// Tap performs a tap action (mobile-like tap) on the specified selector
func (c *Controller) Tap(selector string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := c.newContext(15 * time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Click(selector, chromedp.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("tap failed on selector '%s': %w", selector, err)
	}

	return nil
}

// Type enters text into an input field
func (c *Controller) Type(selector string, text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := c.newContext(15 * time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Clear(selector),
		chromedp.SendKeys(selector, text, chromedp.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("type failed on selector '%s': %w", selector, err)
	}

	return nil
}

// Scroll scrolls the page by the specified amount
func (c *Controller) Scroll(x, y int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := c.newContext(10 * time.Second)
	defer cancel()

	script := fmt.Sprintf("window.scrollBy(%d, %d);", x, y)
	err := chromedp.Run(ctx,
		chromedp.Evaluate(script, nil),
		chromedp.Sleep(200*time.Millisecond),
	)

	if err != nil {
		return fmt.Errorf("scroll failed: %w", err)
	}

	return nil
}

// ScrollToElement scrolls to a specific element
func (c *Controller) ScrollToElement(selector string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := c.newContext(15 * time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.ScrollIntoView(selector, chromedp.ByQuery),
		chromedp.Sleep(200*time.Millisecond),
	)

	if err != nil {
		return fmt.Errorf("scroll to element failed for selector '%s': %w", selector, err)
	}

	return nil
}

// Swipe performs a swipe action (scroll with animation)
func (c *Controller) Swipe(direction string, distance int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := c.newContext(10 * time.Second)
	defer cancel()

	var script string
	switch direction {
	case "up":
		script = fmt.Sprintf("window.scrollBy({top: -%d, behavior: 'smooth'});", distance)
	case "down":
		script = fmt.Sprintf("window.scrollBy({top: %d, behavior: 'smooth'});", distance)
	case "left":
		script = fmt.Sprintf("window.scrollBy({left: -%d, behavior: 'smooth'});", distance)
	case "right":
		script = fmt.Sprintf("window.scrollBy({left: %d, behavior: 'smooth'});", distance)
	default:
		return fmt.Errorf("invalid swipe direction: %s", direction)
	}

	err := chromedp.Run(ctx,
		chromedp.Evaluate(script, nil),
		chromedp.Sleep(500*time.Millisecond),
	)

	if err != nil {
		return fmt.Errorf("swipe failed: %w", err)
	}

	return nil
}

// WaitForElement waits for an element to be visible
func (c *Controller) WaitForElement(selector string, timeout time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := c.newContext(timeout)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("wait for element failed for selector '%s': %w", selector, err)
	}

	return nil
}

// GetElementText retrieves text from an element
func (c *Controller) GetElementText(selector string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var text string
	ctx, cancel := c.newContext(15 * time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Text(selector, &text, chromedp.ByQuery),
	)

	if err != nil {
		return "", fmt.Errorf("get element text failed for selector '%s': %w", selector, err)
	}

	return text, nil
}

// ExecuteScript executes custom JavaScript
func (c *Controller) ExecuteScript(script string) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var result interface{}
	ctx, cancel := c.newContext(15 * time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.Evaluate(script, &result),
	)

	if err != nil {
		return nil, fmt.Errorf("execute script failed: %w", err)
	}

	return result, nil
}