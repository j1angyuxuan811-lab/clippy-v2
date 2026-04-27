package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	api.HandleFunc("/clips/image", s.handleImageUpload).Methods("POST")
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

	// Add absolute image paths for WebView access
	type ClipJSON struct {
		ID           int    `json:"id"`
		Content      string `json:"content"`
		ContentType  string `json:"content_type"`
		ImagePath    string `json:"image_path,omitempty"`
		ImageAbsPath string `json:"image_abs_path,omitempty"`
		Tags         string `json:"tags"`
		IsPinned     bool   `json:"pinned"`
		HotCount     int    `json:"hot_count"`
		CreatedAt    string `json:"created_at"`
	}
	result := make([]ClipJSON, 0, len(clips))
	for _, c := range clips {
		cj := ClipJSON{
			ID: c.ID, Content: c.Content, ContentType: c.ContentType,
			ImagePath: c.ImagePath, Tags: c.Tags, IsPinned: c.IsPinned,
			HotCount: c.HotCount, CreatedAt: c.CreatedAt,
		}
		if c.ImagePath != "" {
			absPath := filepath.Join(s.imagesDir, filepath.Base(c.ImagePath))
			cj.ImageAbsPath = absPath
		}
		result = append(result, cj)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"clips": result,
		"count": len(result),
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

func (s *Server) handleImageUpload(w http.ResponseWriter, r *http.Request) {
	// Max 10MB
	r.ParseMultipartForm(10 << 20)

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing image file", 400)
		return
	}
	defer file.Close()

	// Validate extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" && ext != ".webp" {
		http.Error(w, "unsupported image type", 400)
		return
	}

	// Save with unique name
	filename := fmt.Sprintf("clip_%d%s", time.Now().UnixNano(), ext)
	dst := filepath.Join(s.imagesDir, filename)

	out, err := os.Create(dst)
	if err != nil {
		http.Error(w, "failed to save image", 500)
		return
	}
	defer out.Close()
	io.Copy(out, file)

	// Get file size for logging
	info, _ := os.Stat(dst)
	sizeKB := float64(info.Size()) / 1024

	// Store in DB
	content := "[图片]"
	item, err := s.store.Create(content, "image", filename)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if item == nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "duplicate"})
		return
	}

	log.Printf("🖼️ Image received: %s (%.1f KB)", filename, sizeKB)
	json.NewEncoder(w).Encode(item)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	err := s.store.Delete(id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (s *Server) handlePin(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	err := s.store.TogglePin(id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "toggled"})
}

// handleCopy - returns clip data. For images, also returns the full image path
// so Swift can read it and put it on the system clipboard via NSPasteboard.
func (s *Server) handleCopy(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	item, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	// For image clips, return the absolute image file path
	response := map[string]interface{}{
		"id":           item.ID,
		"content":      item.Content,
		"content_type": item.ContentType,
		"image_path":   item.ImagePath,
		"pinned":       item.IsPinned,
		"created_at":   item.CreatedAt,
	}

	if item.ContentType == "image" && item.ImagePath != "" {
		absPath := filepath.Join(s.imagesDir, filepath.Base(item.ImagePath))
		response["image_abs_path"] = absPath
		log.Printf("📋 Copy requested for image: %s", absPath)
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	clips, err := s.store.List(200)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if clips == nil {
		clips = []db.Item{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=clippy-export.json")
	json.NewEncoder(w).Encode(clips)
}

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	filename := mux.Vars(r)["filename"]
	// Strip any directory prefix (e.g., "data/images/xxx.png" -> "xxx.png")
	filename = filepath.Base(filename)
	path := filepath.Join(s.imagesDir, filename)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.Error(w, "not found", 404)
		return
	}

	http.ServeFile(w, r, path)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func detectType(content string) string {
	lower := strings.ToLower(strings.TrimSpace(content))

	// URL
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return "link"
	}

	// Code indicators
	codeIndicators := []string{"func ", "function ", "def ", "class ", "import ", "package ",
		"const ", "let ", "var ", "{", "}", "//", "/*", "*/", "=>"}
	for _, ind := range codeIndicators {
		if strings.Contains(lower, ind) {
			return "code"
		}
	}

	return "text"
}
