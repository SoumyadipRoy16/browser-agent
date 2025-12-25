package server

import (
	"browser-agent/internal/browser"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type Server struct {
	router      *mux.Router
	browser     *browser.Controller
	httpServer  *http.Server
	wsUpgrader  websocket.Upgrader
}

type ActionRequest struct {
	Action   string                 `json:"action"`
	Params   map[string]interface{} `json:"params"`
}

type ActionResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewServer(browserCtrl *browser.Controller, addr string) *Server {
	s := &Server{
		router:  mux.NewRouter(),
		browser: browserCtrl,
		wsUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
	}

	s.setupRoutes()

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s
}

func (s *Server) setupRoutes() {
	// Serve static files
	s.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	
	// Main page
	s.router.HandleFunc("/", s.handleIndex).Methods("GET")
	
	// API endpoints
	s.router.HandleFunc("/api/navigate", s.handleNavigate).Methods("POST")
	s.router.HandleFunc("/api/action", s.handleAction).Methods("POST")
	s.router.HandleFunc("/api/screenshot", s.handleScreenshot).Methods("GET")
	s.router.HandleFunc("/api/status", s.handleStatus).Methods("GET")
	
	// WebSocket endpoint
	s.router.HandleFunc("/ws", s.handleWebSocket)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/static/index.html")
}

func (s *Server) handleNavigate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		s.sendError(w, "URL is required", http.StatusBadRequest)
		return
	}

	if err := s.browser.Navigate(req.URL); err != nil {
		s.sendError(w, fmt.Sprintf("Navigation failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendSuccess(w, "Navigation successful", nil)
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var err error
	var result interface{}

	switch req.Action {
	case "click":
		selector, ok := req.Params["selector"].(string)
		if !ok || selector == "" {
			s.sendError(w, "selector parameter is required", http.StatusBadRequest)
			return
		}
		err = s.browser.Click(selector)

	case "tap":
		selector, ok := req.Params["selector"].(string)
		if !ok || selector == "" {
			s.sendError(w, "selector parameter is required", http.StatusBadRequest)
			return
		}
		err = s.browser.Tap(selector)

	case "type":
		selector, ok := req.Params["selector"].(string)
		text, textOk := req.Params["text"].(string)
		if !ok || !textOk || selector == "" {
			s.sendError(w, "selector and text parameters are required", http.StatusBadRequest)
			return
		}
		err = s.browser.Type(selector, text)

	case "scroll":
		xFloat, xOk := req.Params["x"].(float64)
		yFloat, yOk := req.Params["y"].(float64)
		if !xOk || !yOk {
			s.sendError(w, "x and y parameters are required", http.StatusBadRequest)
			return
		}
		x := int(xFloat)
		y := int(yFloat)
		err = s.browser.Scroll(x, y)

	case "scrollToElement":
		selector, ok := req.Params["selector"].(string)
		if !ok || selector == "" {
			s.sendError(w, "selector parameter is required", http.StatusBadRequest)
			return
		}
		err = s.browser.ScrollToElement(selector)

	case "swipe":
		direction, dirOk := req.Params["direction"].(string)
		distFloat, distOk := req.Params["distance"].(float64)
		if !dirOk || !distOk || direction == "" {
			s.sendError(w, "direction and distance parameters are required", http.StatusBadRequest)
			return
		}
		distance := int(distFloat)
		err = s.browser.Swipe(direction, distance)

	case "getText":
		selector, ok := req.Params["selector"].(string)
		if !ok || selector == "" {
			s.sendError(w, "selector parameter is required", http.StatusBadRequest)
			return
		}
		result, err = s.browser.GetElementText(selector)

	case "executeScript":
		script, ok := req.Params["script"].(string)
		if !ok || script == "" {
			s.sendError(w, "script parameter is required", http.StatusBadRequest)
			return
		}
		result, err = s.browser.ExecuteScript(script)

	default:
		s.sendError(w, fmt.Sprintf("Unknown action: %s", req.Action), http.StatusBadRequest)
		return
	}

	if err != nil {
		s.sendError(w, fmt.Sprintf("Action failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendSuccess(w, "Action completed successfully", result)
}

func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	buf, err := s.browser.GetScreenshot()
	if err != nil {
		s.sendError(w, fmt.Sprintf("Screenshot failed: %v", err), http.StatusInternalServerError)
		return
	}

	encoded := base64.StdEncoding.EncodeToString(buf)
	s.sendSuccess(w, "Screenshot captured", map[string]string{
		"image": encoded,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	url, urlErr := s.browser.GetCurrentURL()
	title, titleErr := s.browser.GetPageTitle()

	if urlErr != nil {
		url = "Error retrieving URL"
	}
	if titleErr != nil {
		title = "Error retrieving title"
	}

	s.sendSuccess(w, "Status retrieved", map[string]interface{}{
		"url":       url,
		"title":     title,
		"navigated": s.browser.IsNavigated(),
	})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Println("WebSocket client connected")

	for {
		var req ActionRequest
		err := conn.ReadJSON(&req)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		response := ActionResponse{Success: true, Message: "Action completed"}

		switch req.Action {
		case "click":
			if selector, ok := req.Params["selector"].(string); ok && selector != "" {
				if err := s.browser.Click(selector); err != nil {
					response.Success = false
					response.Message = err.Error()
				}
			} else {
				response.Success = false
				response.Message = "Invalid selector"
			}

		case "scroll":
			xFloat, xOk := req.Params["x"].(float64)
			yFloat, yOk := req.Params["y"].(float64)
			if xOk && yOk {
				x := int(xFloat)
				y := int(yFloat)
				if err := s.browser.Scroll(x, y); err != nil {
					response.Success = false
					response.Message = err.Error()
				}
			} else {
				response.Success = false
				response.Message = "Invalid scroll parameters"
			}

		default:
			response.Success = false
			response.Message = "Unknown action"
		}

		if err := conn.WriteJSON(response); err != nil {
			log.Printf("WebSocket write error: %v", err)
			break
		}
	}

	log.Println("WebSocket client disconnected")
}

func (s *Server) sendSuccess(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ActionResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func (s *Server) sendError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ActionResponse{
		Success: false,
		Message: message,
	})
}

func (s *Server) Start() error {
	log.Printf("Server starting on %s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down server...")
	return s.httpServer.Shutdown(ctx)
}