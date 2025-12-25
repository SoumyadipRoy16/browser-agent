// Global state
let currentStatus = 'Ready';

// Initialize on page load
document.addEventListener('DOMContentLoaded', function() {
    log('Browser Agent Controller initialized', 'info');
    updateStatus();
    
    // Update status every 5 seconds
    setInterval(updateStatus, 5000);
});

// Logging function
function log(message, type = 'info') {
    const logContainer = document.getElementById('log');
    const timestamp = new Date().toLocaleTimeString();
    const entry = document.createElement('div');
    entry.className = 'log-entry';
    
    let className = 'log-info';
    let icon = 'ℹ️';
    
    if (type === 'success') {
        className = 'log-success';
        icon = '✅';
    } else if (type === 'error') {
        className = 'log-error';
        icon = '❌';
    }
    
    entry.innerHTML = `<span class="log-time">[${timestamp}]</span><span class="${className}">${icon} ${message}</span>`;
    logContainer.insertBefore(entry, logContainer.firstChild);
    
    // Keep only last 50 entries
    while (logContainer.children.length > 50) {
        logContainer.removeChild(logContainer.lastChild);
    }
}

function clearLog() {
    document.getElementById('log').innerHTML = '';
    log('Log cleared', 'info');
}

// API call wrapper
async function apiCall(endpoint, method = 'GET', body = null) {
    try {
        const options = {
            method: method,
            headers: {
                'Content-Type': 'application/json',
            }
        };
        
        if (body) {
            options.body = JSON.stringify(body);
        }
        
        const response = await fetch(endpoint, options);
        const data = await response.json();
        
        if (!data.success) {
            throw new Error(data.message);
        }
        
        return data;
    } catch (error) {
        throw error;
    }
}

// Update status
async function updateStatus() {
    try {
        const data = await apiCall('/api/status');
        
        if (data.data.navigated) {
            document.getElementById('status').textContent = 'Active';
            document.getElementById('currentUrl').textContent = data.data.url || 'Unknown';
            document.getElementById('currentUrl').title = data.data.title || '';
        } else {
            document.getElementById('status').textContent = 'Ready';
            document.getElementById('currentUrl').textContent = 'Not navigated';
        }
    } catch (error) {
        document.getElementById('status').textContent = 'Error';
        console.error('Status update failed:', error);
    }
}

// Navigation
async function navigate() {
    const url = document.getElementById('urlInput').value.trim();
    
    if (!url) {
        log('Please enter a URL', 'error');
        return;
    }
    
    // Add https:// if not present
    const fullUrl = url.startsWith('http') ? url : 'https://' + url;
    
    log(`Navigating to ${fullUrl}...`, 'info');
    
    try {
        await apiCall('/api/navigate', 'POST', { url: fullUrl });
        log(`Successfully navigated to ${fullUrl}`, 'success');
        setTimeout(updateStatus, 1000);
    } catch (error) {
        log(`Navigation failed: ${error.message}`, 'error');
    }
}

// Click element
async function clickElement() {
    const selector = document.getElementById('clickSelector').value.trim();
    
    if (!selector) {
        log('Please enter a selector', 'error');
        return;
    }
    
    log(`Clicking element: ${selector}`, 'info');
    
    try {
        await apiCall('/api/action', 'POST', {
            action: 'click',
            params: { selector: selector }
        });
        log(`Successfully clicked: ${selector}`, 'success');
    } catch (error) {
        log(`Click failed: ${error.message}`, 'error');
    }
}

// Tap element
async function tapElement() {
    const selector = document.getElementById('tapSelector').value.trim();
    
    if (!selector) {
        log('Please enter a selector', 'error');
        return;
    }
    
    log(`Tapping element: ${selector}`, 'info');
    
    try {
        await apiCall('/api/action', 'POST', {
            action: 'tap',
            params: { selector: selector }
        });
        log(`Successfully tapped: ${selector}`, 'success');
    } catch (error) {
        log(`Tap failed: ${error.message}`, 'error');
    }
}

// Type text
async function typeText() {
    const selector = document.getElementById('typeSelector').value.trim();
    const text = document.getElementById('typeText').value;
    
    if (!selector || !text) {
        log('Please enter both selector and text', 'error');
        return;
    }
    
    log(`Typing into ${selector}: "${text}"`, 'info');
    
    try {
        await apiCall('/api/action', 'POST', {
            action: 'type',
            params: { 
                selector: selector,
                text: text
            }
        });
        log(`Successfully typed text into: ${selector}`, 'success');
    } catch (error) {
        log(`Type failed: ${error.message}`, 'error');
    }
}

// Scroll page
async function scrollPage() {
    const x = parseInt(document.getElementById('scrollX').value) || 0;
    const y = parseInt(document.getElementById('scrollY').value) || 0;
    
    log(`Scrolling page by (${x}, ${y})`, 'info');
    
    try {
        await apiCall('/api/action', 'POST', {
            action: 'scroll',
            params: { x: x, y: y }
        });
        log(`Successfully scrolled by (${x}, ${y})`, 'success');
    } catch (error) {
        log(`Scroll failed: ${error.message}`, 'error');
    }
}

// Quick scroll
async function quickScroll(direction) {
    const distance = direction === 'up' ? -300 : 300;
    
    log(`Quick scrolling ${direction}`, 'info');
    
    try {
        await apiCall('/api/action', 'POST', {
            action: 'scroll',
            params: { x: 0, y: distance }
        });
        log(`Quick scroll ${direction} completed`, 'success');
    } catch (error) {
        log(`Quick scroll failed: ${error.message}`, 'error');
    }
}

// Swipe
async function swipe(direction) {
    log(`Swiping ${direction}`, 'info');
    
    try {
        await apiCall('/api/action', 'POST', {
            action: 'swipe',
            params: { 
                direction: direction,
                distance: 400
            }
        });
        log(`Swipe ${direction} completed`, 'success');
    } catch (error) {
        log(`Swipe failed: ${error.message}`, 'error');
    }
}

// Scroll to element
async function scrollToElement() {
    const selector = document.getElementById('scrollToSelector').value.trim();
    
    if (!selector) {
        log('Please enter a selector', 'error');
        return;
    }
    
    log(`Scrolling to element: ${selector}`, 'info');
    
    try {
        await apiCall('/api/action', 'POST', {
            action: 'scrollToElement',
            params: { selector: selector }
        });
        log(`Successfully scrolled to: ${selector}`, 'success');
    } catch (error) {
        log(`Scroll to element failed: ${error.message}`, 'error');
    }
}

// Get element text
async function getElementText() {
    const selector = document.getElementById('getTextSelector').value.trim();
    const resultBox = document.getElementById('textResult');
    
    if (!selector) {
        log('Please enter a selector', 'error');
        return;
    }
    
    log(`Getting text from: ${selector}`, 'info');
    
    try {
        const data = await apiCall('/api/action', 'POST', {
            action: 'getText',
            params: { selector: selector }
        });
        
        resultBox.textContent = data.data || '(empty)';
        resultBox.classList.add('active');
        log(`Retrieved text from: ${selector}`, 'success');
    } catch (error) {
        log(`Get text failed: ${error.message}`, 'error');
        resultBox.classList.remove('active');
    }
}

// Execute script
async function executeScript() {
    const script = document.getElementById('scriptInput').value.trim();
    const resultBox = document.getElementById('scriptResult');
    
    if (!script) {
        log('Please enter a script', 'error');
        return;
    }
    
    log(`Executing script...`, 'info');
    
    try {
        const data = await apiCall('/api/action', 'POST', {
            action: 'executeScript',
            params: { script: script }
        });
        
        resultBox.textContent = JSON.stringify(data.data, null, 2);
        resultBox.classList.add('active');
        log(`Script executed successfully`, 'success');
    } catch (error) {
        log(`Script execution failed: ${error.message}`, 'error');
        resultBox.classList.remove('active');
    }
}

// Take screenshot
async function takeScreenshot() {
    log('Capturing screenshot...', 'info');
    
    try {
        const data = await apiCall('/api/screenshot');
        
        const container = document.getElementById('screenshotContainer');
        container.innerHTML = `<img src="data:image/png;base64,${data.data.image}" alt="Screenshot">`;
        
        log('Screenshot captured successfully', 'success');
    } catch (error) {
        log(`Screenshot failed: ${error.message}`, 'error');
    }
}

// Keyboard shortcuts
document.addEventListener('keydown', function(e) {
    // Ctrl/Cmd + Enter to navigate
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        if (document.activeElement.id === 'urlInput') {
            navigate();
        }
    }
});