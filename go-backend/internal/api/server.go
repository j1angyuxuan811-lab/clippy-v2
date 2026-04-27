package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"clippy-backend/internal/db"

	"github.com/gorilla/mux"
)

// Server handles HTTP API requests
type Server struct {
	store     *db.Store
	server    *http.Server
	staticDir string
}

// NewServer creates a new API server
func NewServer(store *db.Store, addr string, staticDir string) *Server {
	s := &Server{store: store, staticDir: staticDir}

	router := mux.NewRouter()
	router.Use(s.corsMiddleware)
	router.Use(s.jsonMiddleware)

	// API routes
	api := router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/health", s.handleHealth).Methods("GET")
	api.HandleFunc("/clips", s.handleListItems).Methods("GET")
	api.HandleFunc("/clips/{id}/pin", s.handleTogglePin).Methods("PUT")
	api.HandleFunc("/clips/{id}/copy", s.handlePaste).Methods("POST")
	api.HandleFunc("/clips/{id}", s.handleDeleteItem).Methods("DELETE")
	api.HandleFunc("/export", s.handleExport).Methods("GET")

	// Static files - serve UI
	fileServer := http.FileServer(http.Dir(s.staticDir))
	router.PathPrefix("/").Handler(fileServer)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("Starting API server on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	log.Println("Shutting down API server...")
	return s.server.Close()
}

// Middleware
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only set JSON content type for API routes
		if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
			w.Header().Set("Content-Type", "application/json")
		}
		next.ServeHTTP(w, r)
	})
}

// Handlers
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleListItems(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	items, err := s.store.ListItems(search, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	if items == nil {
		items = []*db.Item{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"clips": items,
		"count": len(items),
	})
}

func (s *Server) handleTogglePin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	item, err := s.store.TogglePin(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(item)
}

func (s *Server) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	if err := s.store.DeleteItem(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (s *Server) handlePaste(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	// Get the item content
	item, err := s.store.GetItem(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	// Increment access count
	s.store.IncrementAccessCount(id)

	// Note: Actual paste simulation would require CGo to call macOS APIs
	// For MVP, we return the content for the Swift frontend to handle
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ready",
		"content": item.Content,
		"message": "Content ready for paste - use Swift frontend to simulate Cmd+V",
	})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListItems("", 1000)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=clippy-export.json")
	json.NewEncoder(w).Encode(items)
}
