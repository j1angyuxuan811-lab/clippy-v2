package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"clippy-backend/internal/db"

	"github.com/gorilla/mux"
)

type Server struct {
	store     *db.Store
	router    *mux.Router
	imagesDir string
}

func New(store *db.Store, staticDir string, imagesDir string) *Server {
	s := &Server{
		store:     store,
		router:    mux.NewRouter(),
		imagesDir: imagesDir,
	}
	s.routes(staticDir)
	return s
}

func (s *Server) routes(staticDir string) {
	// CORS middleware
	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(200)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// API routes
	api := s.router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/clips", s.handleList).Methods("GET")
	api.HandleFunc("/clips", s.handleCreate).Methods("POST")
	api.HandleFunc("/clips/{id}", s.handleDelete).Methods("DELETE")
	api.HandleFunc("/clips/{id}/pin", s.handlePin).Methods("PUT")
	api.HandleFunc("/clips/{id}/copy", s.handleCopy).Methods("POST")
	api.HandleFunc("/clips/export", s.handleExport).Methods("GET")
	api.HandleFunc("/health", s.handleHealth).Methods("GET")

	// Image serving
	s.router.HandleFunc("/images/{filename}", s.handleImage).Methods("GET")

	// Static UI files
	if staticDir != "" {
		s.router.PathPrefix("/").Handler(http.FileServer(http.Dir(staticDir)))
	}
}

func (s *Server) ListenAndServe(addr string) error {
	log.Printf("🌐 API server at %s", addr)
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	var clips []db.Item
	var err error

	if query != "" {
		clips, err = s.store.Search(query)
	} else {
		clips, err = s.store.List(200)
	}

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if clips == nil {
		clips = []db.Item{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"clips": clips,
		"count": len(clips),
	})
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}

	contentType := detectType(req.Content)
	item, err := s.store.Create(req.Content, contentType, "")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if item == nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "duplicate"})
		return
	}
	json.NewEncoder(w).Encode(item)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	err := s.store.Delete(id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handlePin(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	err := s.store.TogglePin(id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleCopy(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	item, err := s.store.Get(id)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	_ = s.store.IncrementHot(id)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"content": item.Content,
	})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	clips, err := s.store.List(1000)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=clippy-export.json")
	json.NewEncoder(w).Encode(clips)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	filename := mux.Vars(r)["filename"]
	// Prevent path traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		http.Error(w, "invalid filename", 400)
		return
	}
	http.ServeFile(w, r, s.imagesDir+"/"+filename)
}

func detectType(content string) string {
	if strings.HasPrefix(content, "http://") || strings.HasPrefix(content, "https://") {
		return "link"
	}
	keywords := []string{"func ", "function ", "class ", "import ", "const ", "var ", "def ", "SELECT ", "FROM ", "public ", "private ", "async "}
	for _, kw := range keywords {
		if strings.Contains(content, kw) {
			return "code"
		}
	}
	return "text"
}
