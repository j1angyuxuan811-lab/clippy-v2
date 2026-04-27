package db

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type Item struct {
	ID          int    `json:"id"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	ImagePath   string `json:"image_path,omitempty"`
	Tags        string `json:"tags"`
	IsPinned    bool   `json:"is_pinned"`
	HotCount    int    `json:"hot_count"`
	CreatedAt   string `json:"created_at"`
}

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, err
	}

	s := &Store{db: db}
	s.init()
	return s, nil
}

func (s *Store) init() {
	query := `CREATE TABLE IF NOT EXISTS clips (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		content     TEXT NOT NULL,
		content_type TEXT DEFAULT 'text',
		image_path  TEXT DEFAULT '',
		tags        TEXT DEFAULT '',
		is_pinned   BOOLEAN DEFAULT 0,
		hot_count   INTEGER DEFAULT 0,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	_, _ = s.db.Exec(query)

	// Migration: add image_path column if missing
	_, _ = s.db.Exec("ALTER TABLE clips ADD COLUMN image_path TEXT DEFAULT ''")
}

func (s *Store) Create(content string, contentType string, imagePath string) (*Item, error) {
	// Check duplicate (last 10)
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM clips WHERE content = ? AND created_at > datetime('now', '-10 seconds')", content).Scan(&count)
	if count > 0 {
		return nil, nil
	}

	res, err := s.db.Exec(
		"INSERT INTO clips (content, content_type, image_path) VALUES (?, ?, ?)",
		content, contentType, imagePath,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()

	// Auto cleanup
	s.cleanup()

	return s.Get(int(id))
}

func (s *Store) Get(id int) (*Item, error) {
	item := &Item{}
	err := s.db.QueryRow(
		"SELECT id, content, content_type, COALESCE(image_path,''), COALESCE(tags,''), is_pinned, hot_count, created_at FROM clips WHERE id = ?", id,
	).Scan(&item.ID, &item.Content, &item.ContentType, &item.ImagePath, &item.Tags, &item.IsPinned, &item.HotCount, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Store) List(limit int) ([]Item, error) {
	rows, err := s.db.Query(
		"SELECT id, content, content_type, COALESCE(image_path,''), COALESCE(tags,''), is_pinned, hot_count, created_at FROM clips ORDER BY is_pinned DESC, created_at DESC LIMIT ?", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		err := rows.Scan(&item.ID, &item.Content, &item.ContentType, &item.ImagePath, &item.Tags, &item.IsPinned, &item.HotCount, &item.CreatedAt)
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Store) Delete(id int) error {
	// Get image path before deleting
	var imagePath string
	_ = s.db.QueryRow("SELECT COALESCE(image_path,'') FROM clips WHERE id = ?", id).Scan(&imagePath)

	_, err := s.db.Exec("DELETE FROM clips WHERE id = ?", id)

	// Delete associated image file
	if imagePath != "" {
		_ = os.Remove(imagePath)
	}

	return err
}

func (s *Store) TogglePin(id int) error {
	_, err := s.db.Exec("UPDATE clips SET is_pinned = NOT is_pinned WHERE id = ?", id)
	return err
}

func (s *Store) IncrementHot(id int) error {
	_, err := s.db.Exec("UPDATE clips SET hot_count = hot_count + 1 WHERE id = ?", id)
	return err
}

func (s *Store) Search(query string) ([]Item, error) {
	rows, err := s.db.Query(
		"SELECT id, content, content_type, COALESCE(image_path,''), COALESCE(tags,''), is_pinned, hot_count, created_at FROM clips WHERE content LIKE ? ORDER BY is_pinned DESC, created_at DESC LIMIT 50",
		"%"+query+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		err := rows.Scan(&item.ID, &item.Content, &item.ContentType, &item.ImagePath, &item.Tags, &item.IsPinned, &item.HotCount, &item.CreatedAt)
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

// Delete old unpinned items (keep maxItems)
func (s *Store) cleanup() {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM clips").Scan(&count)
	if count <= 1000 {
		return
	}

	// Delete oldest unpinned items
	rows, _ := s.db.Query(
		"SELECT id, COALESCE(image_path,'') FROM clips WHERE is_pinned = 0 ORDER BY created_at ASC LIMIT ?",
		count-1000,
	)
	defer rows.Close()

	var ids []int
	var paths []string
	for rows.Next() {
		var id int
		var path string
		rows.Scan(&id, &path)
		ids = append(ids, id)
		if path != "" {
			paths = append(paths, path)
		}
	}

	for _, id := range ids {
		s.db.Exec("DELETE FROM clips WHERE id = ?", id)
	}
	for _, path := range paths {
		_ = os.Remove(path)
	}

	log.Printf("🧹 Cleaned up %d old items", len(ids))
}

// Clean up orphan images (images on disk not referenced in DB)
func (s *Store) CleanupOrphanImages(imagesDir string) {
	if imagesDir == "" {
		return
	}
	entries, err := os.ReadDir(imagesDir)
	if err != nil {
		return
	}

	// Get all referenced image paths
	rows, _ := s.db.Query("SELECT DISTINCT image_path FROM clips WHERE image_path != ''")
	defer rows.Close()

	referenced := make(map[string]bool)
	for rows.Next() {
		var path string
		rows.Scan(&path)
		referenced[path] = true
	}

	cleaned := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(imagesDir, entry.Name())
		relPath := filepath.Join("data", "images", entry.Name())
		if !referenced[relPath] {
			_ = os.Remove(fullPath)
			cleaned++
		}
	}
	if cleaned > 0 {
		log.Printf("🧹 Cleaned %d orphan images", cleaned)
	}
}

// Total image size
func (s *Store) ImageDirSize(imagesDir string) int64 {
	var size int64
	_ = filepath.Walk(imagesDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// Evict oldest images if total exceeds limit (200MB)
func (s *Store) EnforceImageLimit(imagesDir string, maxBytes int64) {
	size := s.ImageDirSize(imagesDir)
	if size <= maxBytes {
		return
	}

	rows, _ := s.db.Query(
		"SELECT id, COALESCE(image_path,'') FROM clips WHERE image_path != '' AND is_pinned = 0 ORDER BY created_at ASC",
	)
	defer rows.Close()

	for rows.Next() && size > maxBytes {
		var id int
		var path string
		rows.Scan(&id, &path)
		if path != "" {
			if info, err := os.Stat(path); err == nil {
				_ = os.Remove(path)
				size -= info.Size()
				s.db.Exec("UPDATE clips SET image_path = '' WHERE id = ?", id)
				log.Printf("🗑️ Evicted image: %s", filepath.Base(path))
			}
		}
	}
}

func (s *Store) Close() {
	_ = s.db.Close()
}
