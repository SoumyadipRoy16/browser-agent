# Autonomous Browser AI Agent

A production-ready autonomous browser automation agent that executes natural language tasks end-to-end using AI planning and real browser control.

## Features

- **Natural Language Task Execution**: Describe what you want in plain English
- **Dynamic Planning**: AI-powered task decomposition and execution
- **Real Browser Control**: Uses Playwright for actual browser automation
- **Interactive Authentication**: Asks for credentials only when needed
- **Long Task Support**: Handles 50+ step workflows with progress tracking
- **Failure Recovery**: Automatic retry and error handling mechanisms
- **Success Detection**: AI determines when tasks are complete or failed

## Prerequisites

- Go 1.21 or higher
- Gemini API key (free tier available at https://ai.google.dev)
- Internet connection

## Installation

### 1. Clone and Setup

```bash
git clone <repository-url>
cd browser-agent
```

### 2. Initialize Go Module

```bash
go mod init browser-agent
```

### 3. Install Dependencies

```bash
go get github.com/playwright-community/playwright-go@v0.4702.0
go get golang.org/x/term@latest
```

### 4. Install Playwright Browsers

```bash
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install chromium
```

### 5. Set Environment Variable

```bash
export GEMINI_API_KEY="your-gemini-api-key-here"
```

Or create a `.env` file:
```
GEMINI_API_KEY=your-gemini-api-key-here
```

## Building

```bash
go build -o agent cmd/main.go
```

## Usage

### Basic Syntax

```bash
./agent run "your task description"
```

### Example Tasks

#### Simple Task (10-15 steps)
```bash
./agent run "Go to google.com and search for weather in San Francisco"
```

#### Medium Complexity (20-30 steps)
```bash
./agent run "Search for best laptops under 1000 dollars on Amazon and tell me the top 3 results with prices"
```

#### Complex Task (30-50+ steps)
```bash
./agent run "Test order placement flow in amazon.com, try to search and place an order for any detergent, go up to payment screen to test if everything until payment is working"
```

#### Research Task (40+ steps)
```bash
./agent run "Go to Wikipedia, search for artificial intelligence, read the introduction, then navigate to machine learning page and summarize the key differences between supervised and unsupervised learning"
```

#### Data Extraction (50+ steps)
```bash
./agent run "Go to Hacker News, collect the top 10 post titles and their scores, then visit each link and extract the main topic of each article"
```

## How It Works

### Architecture

```
┌─────────────────┐
│  User Command   │
└────────┬────────┘
         │
    ┌────▼─────┐
    │ Planner  │ (Gemini AI)
    └────┬─────┘
         │
    ┌────▼─────────┐
    │  Executor    │ (Browser Control)
    └────┬─────────┘
         │
    ┌────▼──────────┐
    │  Validator    │ (Success Check)
    └───────────────┘
```

### Components

1. **Planner Agent**
   - Decomposes tasks into actionable steps
   - Understands context and dependencies
   - Adapts plan based on execution results

2. **Executor Agent**
   - Controls browser using Playwright
   - Executes individual actions (click, type, navigate)
   - Captures screenshots and page state
   - Handles authentication prompts

3. **Validator Agent**
   - Determines task completion
   - Detects failures and suggests recovery
   - Tracks progress across long workflows

### Long Task Handling

- **Step Limit**: 100 steps maximum (configurable)
- **Progress Tracking**: Each step is logged with status
- **Loop Detection**: Prevents infinite repetition
- **Timeout Management**: Per-step and total timeouts
- **State Persistence**: Maintains context across steps

### Authentication Flow

When the agent needs credentials:

```
Agent: I need to log in. Please provide:
Email: user@example.com
Password: [hidden input]
```

Credentials are used immediately and never stored.

## Configuration

Edit `internal/config/config.go` to modify:

- `MaxSteps`: Maximum steps per task (default: 100)
- `StepTimeout`: Timeout per step (default: 30s)
- `TotalTimeout`: Maximum total execution time (default: 10m)
- `Headless`: Browser visibility (default: false for debugging)

## Project Structure

```
browser-agent/
├── cmd/
│   └── main.go                 # Entry point
├── internal/
│   ├── amazon_agent/
│   │   ├── agent.go           # Core agent logic
│   │   ├── planner.go         # Task planning
│   │   ├── executor.go        # Browser automation
│   │   └── validator.go       # Success validation
│   ├── browser/
│   │   └── browser.go         # Playwright wrapper
│   ├── llm/
│   │   └── gemini.go          # Gemini API client
│   └── config/
│       └── config.go          # Configuration
├── go.mod
├── go.sum
└── README.md
```

## Example Execution Log

```
2024-12-27 10:30:00 [INFO] Starting task: Test Amazon order flow
2024-12-27 10:30:01 [PLAN] Generated 8 initial steps
2024-12-27 10:30:02 [EXEC] Step 1/8: Navigate to amazon.com
2024-12-27 10:30:03 [EXEC] Step 2/8: Search for detergent
2024-12-27 10:30:05 [AUTH] Requesting user email
2024-12-27 10:30:15 [EXEC] Step 3/8: Click sign in
2024-12-27 10:30:20 [EXEC] Step 4/8: Select first product
2024-12-27 10:30:25 [EXEC] Step 5/8: Add to cart
2024-12-27 10:30:30 [EXEC] Step 6/8: Proceed to checkout
2024-12-27 10:30:35 [EXEC] Step 7/8: Navigate to payment screen
2024-12-27 10:30:40 [VALID] Payment screen reached - SUCCESS
2024-12-27 10:30:40 [INFO] Task completed in 40s (7 steps)
```

## Troubleshooting

### Browser doesn't open
```bash
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install chromium
```

### API Key Issues
Verify your key:
```bash
echo $GEMINI_API_KEY
```

### Timeout Errors
Increase timeouts in `config.go`:
```go
StepTimeout: 60 * time.Second
TotalTimeout: 15 * time.Minute
```

### Rate Limiting
Gemini free tier: 15 requests/minute. The agent handles this automatically with exponential backoff.

## Advanced Usage

### Custom Browser Options

Modify `internal/browser/browser.go`:

```go
opts := playwright.BrowserTypeLaunchOptions{
    Headless: playwright.Bool(true),
    SlowMo:   playwright.Float(50),
}
```

### Proxy Configuration

```go
opts := playwright.BrowserTypeLaunchOptions{
    Proxy: &playwright.BrowserTypeLaunchOptionsProxy{
        Server: playwright.String("http://proxy:8080"),
    },
}
```

## API Costs

Gemini Flash 2.0 (free tier):
- Input: Free up to 1M tokens/day
- Output: Free up to 32K tokens/day
- This agent typically uses 2-5K tokens per task

## Limitations

- Visual CAPTCHA not supported
- File downloads require manual handling
- Multi-tab scenarios need explicit planning
- JavaScript-heavy SPAs may need wait strategies

## Development

Run tests:
```bash
go test ./...
```

Build with debug info:
```bash
go build -race -o agent cmd/main.go
```

Enable verbose logging:
```go
log.SetLevel(log.DebugLevel)
```

## License

MIT License - feel free to modify and distribute

## Support

For issues or questions:
1. Check troubleshooting section
2. Review execution logs
3. Enable debug mode for detailed output

---

**Built with ❤️ using Go, Playwright, and Gemini AI**