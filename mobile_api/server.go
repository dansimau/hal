package mobile_api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// StartupScreenConfig represents the configuration for the startup screen
type StartupScreenConfig struct {
	ShowScreen      bool              `json:"show_screen"`
	IsBlocking      bool              `json:"is_blocking"`
	Title           string            `json:"title"`
	Message         string            `json:"message"`
	ButtonText      string            `json:"button_text"`
	ImageURL        string            `json:"image_url,omitempty"`
	BackgroundColor string            `json:"background_color,omitempty"`
	TextColor       string            `json:"text_color,omitempty"`
	ExpiresAt       *time.Time        `json:"expires_at,omitempty"`
	MinAppVersion   string            `json:"min_app_version,omitempty"`
	MaxAppVersion   string            `json:"max_app_version,omitempty"`
	Actions         []ScreenAction    `json:"actions,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// ScreenAction represents an action button on the startup screen
type ScreenAction struct {
	Text     string `json:"text"`
	Type     string `json:"type"` // "dismiss", "redirect", "force_update"
	URL      string `json:"url,omitempty"`
	IsPrimary bool  `json:"is_primary"`
}

// StartupScreenRequest represents the client request for startup screen configuration
type StartupScreenRequest struct {
	AppVersion    string `json:"app_version"`
	Platform      string `json:"platform"` // "ios", "android"
	DeviceID      string `json:"device_id"`
	LastShownAt   *time.Time `json:"last_shown_at,omitempty"`
	UserID        string `json:"user_id,omitempty"`
}

// MobileAPIServer handles mobile app API requests
type MobileAPIServer struct {
	server *http.Server
	router *mux.Router
}

// NewMobileAPIServer creates a new mobile API server
func NewMobileAPIServer(port int) *MobileAPIServer {
	router := mux.NewRouter()
	
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      10 * time.Second,
		ReadTimeout:       30 * time.Second,
	}

	apiServer := &MobileAPIServer{
		server: server,
		router: router,
	}

	apiServer.setupRoutes()
	return apiServer
}

// setupRoutes configures the API routes
func (s *MobileAPIServer) setupRoutes() {
	// Add CORS middleware
	s.router.Use(corsMiddleware)
	
	// API routes
	api := s.router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/startup-screen", s.handleStartupScreen).Methods("POST", "OPTIONS")
	api.HandleFunc("/health", s.handleHealth).Methods("GET")
}

// handleStartupScreen handles the startup screen configuration request
func (s *MobileAPIServer) handleStartupScreen(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		return
	}

	var req StartupScreenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	slog.Info("Startup screen request", 
		"app_version", req.AppVersion,
		"platform", req.Platform,
		"device_id", req.DeviceID)

	// Generate startup screen configuration based on request
	config := s.generateStartupScreenConfig(req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// generateStartupScreenConfig generates the startup screen configuration
// This is where you would implement your business logic for determining
// when to show the startup screen and what content to display
func (s *MobileAPIServer) generateStartupScreenConfig(req StartupScreenRequest) StartupScreenConfig {
	// Example logic - you can customize this based on your needs
	
	// Check if this is an old version that needs a forced update
	if s.isOldVersion(req.AppVersion) {
		return StartupScreenConfig{
			ShowScreen:      true,
			IsBlocking:      true,
			Title:           "Update Required",
			Message:         "A new version of the app is available with important security updates. Please update to continue using the app.",
			ButtonText:      "Update Now",
			BackgroundColor: "#FF6B6B",
			TextColor:       "#FFFFFF",
			Actions: []ScreenAction{
				{
					Text:      "Update Now",
					Type:      "force_update",
					URL:       s.getAppStoreURL(req.Platform),
					IsPrimary: true,
				},
			},
			MinAppVersion: "2.0.0",
			Metadata: map[string]string{
				"update_type": "forced",
				"severity":    "high",
			},
		}
	}

	// Check for promotional or informational content
	if s.shouldShowPromotion(req) {
		expiresAt := time.Now().Add(24 * time.Hour)
		return StartupScreenConfig{
			ShowScreen:      true,
			IsBlocking:      false,
			Title:           "Welcome to HAL Mobile!",
			Message:         "Manage your home automation from anywhere. Set up your devices and create custom automations with ease.",
			ButtonText:      "Get Started",
			ImageURL:        "https://example.com/welcome-image.png",
			BackgroundColor: "#4A90E2",
			TextColor:       "#FFFFFF",
			ExpiresAt:       &expiresAt,
			Actions: []ScreenAction{
				{
					Text:      "Get Started",
					Type:      "dismiss",
					IsPrimary: true,
				},
				{
					Text:      "Learn More",
					Type:      "redirect",
					URL:       "https://example.com/learn-more",
					IsPrimary: false,
				},
			},
			Metadata: map[string]string{
				"campaign_id": "welcome_2024",
				"content_type": "onboarding",
			},
		}
	}

	// Default: no startup screen
	return StartupScreenConfig{
		ShowScreen: false,
	}
}

// isOldVersion checks if the app version is too old and needs an update
func (s *MobileAPIServer) isOldVersion(version string) bool {
	// Implement version comparison logic here
	// For demo purposes, consider versions < "2.0.0" as old
	return version < "2.0.0"
}

// shouldShowPromotion determines if a promotional screen should be shown
func (s *MobileAPIServer) shouldShowPromotion(req StartupScreenRequest) bool {
	// Implement your promotion logic here
	// For demo purposes, show to new users (no last_shown_at)
	return req.LastShownAt == nil
}

// getAppStoreURL returns the appropriate app store URL for the platform
func (s *MobileAPIServer) getAppStoreURL(platform string) string {
	switch platform {
	case "ios":
		return "https://apps.apple.com/app/hal-home-automation/id123456789"
	case "android":
		return "https://play.google.com/store/apps/details?id=com.hal.homeautomation"
	default:
		return "https://hal-app.com/download"
	}
}

// handleHealth provides a health check endpoint
func (s *MobileAPIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// corsMiddleware adds CORS headers to allow requests from mobile apps
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Start starts the mobile API server
func (s *MobileAPIServer) Start() error {
	slog.Info("Starting mobile API server", "addr", s.server.Addr)
	return s.server.ListenAndServe()
}

// Stop stops the mobile API server
func (s *MobileAPIServer) Stop() error {
	slog.Info("Stopping mobile API server")
	return s.server.Close()
}