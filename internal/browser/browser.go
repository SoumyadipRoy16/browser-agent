package browser

import (
	"fmt"
	"time"

	"github.com/playwright-community/playwright-go"
)

type Browser struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	context playwright.BrowserContext
	page    playwright.Page
}

type PageState struct {
	URL     string
	Title   string
	Content string
}

func NewBrowser(headless bool, slowMo float64) (*Browser, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("start playwright: %w", err)
	}

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
		SlowMo:   playwright.Float(slowMo),
	})
	if err != nil {
		pw.Stop()
		return nil, fmt.Errorf("launch browser: %w", err)
	}

	context, err := browser.NewContext()
	if err != nil {
		browser.Close()
		pw.Stop()
		return nil, fmt.Errorf("create context: %w", err)
	}

	page, err := context.NewPage()
	if err != nil {
		context.Close()
		browser.Close()
		pw.Stop()
		return nil, fmt.Errorf("create page: %w", err)
	}

	return &Browser{
		pw:      pw,
		browser: browser,
		context: context,
		page:    page,
	}, nil
}

func (b *Browser) Navigate(url string) error {
	_, err := b.page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(60000),
	})
	if err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
	return nil
}

func (b *Browser) Click(selector string) error {
	return b.page.Click(selector, playwright.PageClickOptions{
		Timeout: playwright.Float(10000),
	})
}

func (b *Browser) Type(selector string, text string) error {
	return b.page.Fill(selector, text)
}

func (b *Browser) Press(selector string, key string) error {
	return b.page.Press(selector, key)
}

func (b *Browser) WaitForSelector(selector string, timeout time.Duration) error {
	_, err := b.page.WaitForSelector(selector, playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(float64(timeout.Milliseconds())),
	})
	return err
}

func (b *Browser) GetText(selector string) (string, error) {
	element, err := b.page.QuerySelector(selector)
	if err != nil {
		return "", err
	}
	if element == nil {
		return "", fmt.Errorf("element not found")
	}
	text, err := element.TextContent()
	if err != nil {
		return "", err
	}
	return text, nil
}

func (b *Browser) GetPageState() (*PageState, error) {
	url := b.page.URL()
	title, err := b.page.Title()
	if err != nil {
		return nil, err
	}

	body, err := b.page.QuerySelector("body")
	if err != nil {
		return nil, err
	}

	var content string
	if body != nil {
		content, err = body.TextContent()
		if err != nil {
			content = ""
		}
	}

	return &PageState{
		URL:     url,
		Title:   title,
		Content: content,
	}, nil
}

func (b *Browser) Screenshot() ([]byte, error) {
	return b.page.Screenshot()
}

func (b *Browser) Evaluate(script string) (interface{}, error) {
	return b.page.Evaluate(script)
}

func (b *Browser) Close() error {
	if b.page != nil {
		b.page.Close()
	}
	if b.context != nil {
		b.context.Close()
	}
	if b.browser != nil {
		b.browser.Close()
	}
	if b.pw != nil {
		return b.pw.Stop()
	}
	return nil
}