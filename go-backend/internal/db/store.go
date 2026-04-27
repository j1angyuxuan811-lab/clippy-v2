package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Item represents a clipboard item in the database
type Item struct {
	ID          int64     `json:"id"`
	Content     string    `json:"content"`
	ContentType string    `json:"content_type"`
	Pinned      bool      `json:"pinned"`
	Tags        string    `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	AccessCount int       `json:"access_count"`
}

// Store manages clipboard items in SQLite
type Store struct {
	db         *sql.DB
	maxItems   int
}

// NewStore creates a new database store
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create table if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			content_type TEXT DEFAULT 'text',
			pinned INTEGER DEFAULT 0,
			tags TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			access_count INTEGER DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_items_created_at ON items(created_at);
		CREATE INDEX IF NOT EXISTS idx_items_pinned ON items(pinned);
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	log.Println("Database initialized successfully")
	return &Store{db: db, maxItems: 1000}, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// AddItem adds a new clipboard item
func (s *Store) AddItem(content string) (*Item, error) {
	// Check for duplicates (same content in last 5 seconds)
	var count int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM items WHERE content = ? AND created_at > datetime('now', '-5 seconds')",
		content,
	).Scan(&count)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, nil // Duplicate, skip
	}

	// Insert new item
	result, err := s.db.Exec(
		"INSERT INTO items (content, content_type) VALUES (?, 'text')",
		content,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert item: %w", err)
	}

	id, _ := result.LastInsertId()

	// Cleanup old items if exceeding max (keep pinned items)
	if err := s.cleanup(); err != nil {
		log.Printf("Warning: cleanup failed: %v", err)
	}

	return s.GetItem(id)
}

// GetItem retrieves a single item by ID
func (s *Store) GetItem(id int64) (*Item, error) {
	item := &Item{}
	err := s.db.QueryRow(
		"SELECT id, content, content_type, pinned, tags, created_at, access_count FROM items WHERE id = ?",
		id,
	).Scan(&item.ID, &item.Content, &item.ContentType, &item.Pinned, &item.Tags, &item.CreatedAt, &item.AccessCount)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// ListItems returns clipboard items with optional search
func (s *Store) ListItems(search string, limit int) ([]*Item, error) {
	if limit <= 0 {
		limit = 50
	}

	var rows *sql.Rows
	var err error

	if search != "" {
		rows, err = s.db.Query(
			"SELECT id, content, content_type, pinned, tags, created_at, access_count FROM items WHERE content LIKE ? ORDER BY pinned DESC, created_at DESC LIMIT ?",
			"%"+search+"%",
			limit,
		)
	} else {
		rows, err = s.db.Query(
			"SELECT id, content, content_type, pinned, tags, created_at, access_count FROM items ORDER BY pinned DESC, created_at DESC LIMIT ?",
			limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*Item
	for rows.Next() {
		item := &Item{}
		err := rows.Scan(&item.ID, &item.Content, &item.ContentType, &item.Pinned, &item.Tags, &item.CreatedAt, &item.AccessCount)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// TogglePin toggles the pinned status of an item
func (s *Store) TogglePin(id int64) (*Item, error) {
	_, err := s.db.Exec(
		"UPDATE items SET pinned = CASE WHEN pinned = 1 THEN 0 ELSE 1 END WHERE id = ?",
		id,
	)
	if err != nil {
		return nil, err
	}
	return s.GetItem(id)
}

// DeleteItem removes an item
func (s *Store) DeleteItem(id int64) error {
	result, err := s.db.Exec("DELETE FROM items WHERE id = ?", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("item not found")
	}
	return nil
}

// IncrementAccessCount increments the access counter for an item
func (s *Store) IncrementAccessCount(id int64) error {
	_, err := s.db.Exec("UPDATE items SET access_count = access_count + 1 WHERE id = ?", id)
	return err
}

// cleanup removes old non-pinned items if exceeding maxItems
func (s *Store) cleanup() error {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	if err != nil {
		return err
	}

	if count > s.maxItems {
		excess := count - s.maxItems
		_, err = s.db.Exec(`
			DELETE FROM items WHERE id IN (
				SELECT id FROM items WHERE pinned = 0 ORDER BY created_at ASC LIMIT ?
			)
		`, excess)
		return err
	}
	return nil
}
