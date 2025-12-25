package browser

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

type Controller struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	isNavigated bool
}

func NewController(parentCtx context.Context) (*Controller, error) {
    // Create allocator context with options
    opts := append(chromedp.DefaultExecAllocatorOptions[:],
        chromedp.Flag("headless", false),
        chromedp.Flag("disable-blink-features", "AutomationControlled"),
        chromedp.WindowSize(1280, 720),
        chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
    )

    allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)

    controller := &Controller{
        allocCtx:    allocCtx,
        allocCancel: allocCancel,
        isNavigated: false,
    }

    // Create the browser context but don't cancel it
    // This will keep the browser open
    ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
    
    // Run an empty task just to start the browser
    if err := chromedp.Run(ctx); err != nil {
        allocCancel()
        cancel()
        return nil, fmt.Errorf("failed to initialize browser: %w", err)
    }
    
    // Store the context and cancel function as fields so browser stays open
    controller.ctx = ctx
    controller.cancel = cancel

    return controller, nil
}

func (c *Controller) newContext(timeout time.Duration) (context.Context, context.CancelFunc) {
    // Use the existing browser context instead of creating a new one
    ctx, cancel := context.WithTimeout(c.ctx, timeout)
    
    return ctx, cancel
}

func (c *Controller) Navigate(url string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := c.newContext(30 * time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
	)

	if err != nil {
		return fmt.Errorf("navigation failed: %w", err)
	}

	c.isNavigated = true
	return nil
}

func (c *Controller) GetCurrentURL() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var url string
	ctx, cancel := c.newContext(5 * time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.Location(&url),
	)

	if err != nil {
		return "", fmt.Errorf("failed to get current URL: %w", err)
	}

	return url, nil
}

func (c *Controller) GetScreenshot() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var buf []byte
	ctx, cancel := c.newContext(15 * time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.Sleep(500*time.Millisecond),
		chromedp.CaptureScreenshot(&buf),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to capture screenshot: %w", err)
	}

	return buf, nil
}

func (c *Controller) GetPageTitle() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var title string
	ctx, cancel := c.newContext(5 * time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.Title(&title),
	)

	if err != nil {
		return "", fmt.Errorf("failed to get page title: %w", err)
	}

	return title, nil
}

func (c *Controller) IsNavigated() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.isNavigated
}

func (c *Controller) Close() {
    c.mu.Lock()
    defer c.mu.Unlock()

    if c.cancel != nil {
        c.cancel()
    }
    
    if c.allocCancel != nil {
        c.allocCancel()
    }
}