# Browser Agent - Golang Browser Automation Prototype

A powerful browser automation agent built with Go that provides a web-based interface for controlling browser actions including clicking, tapping, scrolling, and more.

## Features

- 🌐 **Browser Control**: Full control over Chrome/Chromium browser
- 🖱️ **UI Operations**: Click, tap, type, scroll, and swipe actions
- 📸 **Screenshots**: Capture browser screenshots on demand
- 🎯 **Element Targeting**: CSS selector-based element interaction
- 💻 **JavaScript Execution**: Run custom JavaScript in the browser
- 🎨 **Modern UI**: Clean, responsive web interface
- 🔄 **Real-time Updates**: Live status monitoring
- 📝 **Activity Logging**: Track all actions and results

## Prerequisites

- Go 1.19 or higher
- Chrome or Chromium browser installed
- Git (for cloning the repository)

## Installation & Setup

### Step 1: Clone and Initialize

```bash
# Create project directory
mkdir browser-agent
cd browser-agent
```

### Step 2: Install Dependencies

```bash
go get github.com/chromedp/chromedp
go get github.com/gorilla/websocket
go get github.com/gorilla/mux
```

### Step 3: Build the Project

```bash
# Download all dependencies
go mod tidy

# Build the application
go build -o browser-agent cmd/agent/main.go
```

## Running the Application

### Option 1: Run Directly
```bash
go run cmd/agent/main.go
```

### Option 2: Run Built Binary
```bash
./browser-agent
```

The server will start on `http://localhost:8080`

## Usage

### Web Interface

1. Open your browser and navigate to `http://localhost:8080`
2. Enter a URL in the navigation field and click "Navigate"
3. Use the various action panels to control the browser:
   - **Basic Actions**: Click, tap, and type on elements
   - **Scroll & Swipe**: Control page scrolling
   - **Advanced Actions**: Get element text and execute JavaScript
   - **Screenshot**: Capture the current browser view

### API Endpoints

#### Navigate to URL
```bash
POST /api/navigate
Content-Type: application/json

{
  "url": "https://example.com"
}
```

#### Perform Action
```bash
POST /api/action
Content-Type: application/json

{
  "action": "click",
  "params": {
    "selector": "button.submit"
  }
}
```

**Available Actions:**
- `click` - Click an element
- `tap` - Tap an element (mobile-like)
- `type` - Type text into an input
- `scroll` - Scroll the page
- `scrollToElement` - Scroll to a specific element
- `swipe` - Swipe in a direction
- `getText` - Get element text
- `executeScript` - Run JavaScript

#### Get Screenshot
```bash
GET /api/screenshot
```

#### Get Status
```bash
GET /api/status
```

## Project Structure

```
browser-agent/
├── cmd/
│   └── agent/
│       └── main.go              # Application entry point
├── internal/
│   ├── browser/
│   │   ├── controller.go        # Browser control logic
│   │   └── actions.go           # Browser action implementations
│   └── server/
│       └── server.go            # HTTP server and API handlers
├── web/
│   └── static/
│       ├── index.html           # Web UI
│       ├── styles.css           # Styling
│       └── app.js               # Frontend JavaScript
├── go.mod                       # Go module definition
├── go.sum                       # Dependency checksums
└── README.md                    # This file
```

## Example Usage

### Navigate and Click
```javascript
// Navigate to a website
fetch('/api/navigate', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ url: 'https://example.com' })
});

// Click a button
fetch('/api/action', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    action: 'click',
    params: { selector: 'button#submit' }
  })
});
```

### Type Text
```javascript
fetch('/api/action', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    action: 'type',
    params: {
      selector: 'input[name="search"]',
      text: 'Hello World'
    }
  })
});
```

### Execute JavaScript
```javascript
fetch('/api/action', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    action: 'executeScript',
    params: { script: 'return document.title;' }
  })
});
```

## Development

### Adding New Actions

1. Add the action method in `internal/browser/actions.go`
2. Add the API handler case in `internal/server/server.go`
3. Update the frontend in `web/static/app.js` and `index.html`

### Debugging

The application logs all activities to the console. Use the browser developer tools to debug frontend issues.

## Troubleshooting

### Chrome/Chromium Not Found
Ensure Chrome or Chromium is installed and accessible in your system PATH.

### Port Already in Use
Change the port in `cmd/agent/main.go`:
```go
srv := server.NewServer(browserCtrl, ":8081")
```

### Dependencies Not Found
Run `go mod tidy` to download all required dependencies.

## Safety & Limitations

- This is a prototype for development/testing purposes
- Be cautious when automating interactions with websites
- Respect robots.txt and website terms of service
- Rate limiting is not implemented - use responsibly

## Future Enhancements

- [ ] Multiple browser instances
- [ ] Session management
- [ ] Network interception
- [ ] File upload/download
- [ ] Cookie management
- [ ] Proxy support
- [ ] Recording and playback
- [ ] Headless mode toggle

## License

This is a prototype project for demonstration purposes.

## Contributing

This is a work trial project. Feedback and suggestions are welcome!

## Support

For issues or questions, please check the logs and ensure all dependencies are properly installed.
