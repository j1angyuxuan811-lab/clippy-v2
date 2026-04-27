package clipboard

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"clippy-backend/internal/db"
)

type Monitor struct {
	store       *db.Store
	interval    time.Duration
	lastHash    string
	imagesDir   string
}

func New(store *db.Store, imagesDir string) *Monitor {
	_ = os.MkdirAll(imagesDir, 0755)
	return &Monitor{
		store:     store,
		interval:  800 * time.Millisecond,
		imagesDir: imagesDir,
	}
}

func (m *Monitor) Start() {
	log.Println("📋 Clipboard monitor started")
	m.check() // first scan
	for {
		time.Sleep(m.interval)
		m.check()
	}
}

func (m *Monitor) check() {
	// 1. Try image first (more specific)
	if m.checkImage() {
		return
	}
	// 2. Try text
	m.checkText()
}

func (m *Monitor) checkText() {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		return
	}
	text := strings.TrimSpace(string(out))
	if text == "" || len(text) > 100_000 {
		return
	}

	hash := hashStr(text)
	if hash == m.lastHash {
		return
	}
	m.lastHash = hash
	m.store.Create(text, "text", "")
}

func (m *Monitor) checkImage() bool {
	// Check if clipboard has image data
	script := `try
	tell application "System Events"
		set cbInfo to the clipboard info
		repeat with itemInfo in cbInfo
			if itemInfo contains "TIFF" or itemInfo contains "PNGf" or itemInfo contains "GIFf" or itemInfo contains "JPEG" then
				return "yes"
			end if
		end repeat
	end try
	return "no"
end try`
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return false
	}
	if strings.TrimSpace(string(out)) != "yes" {
		return false
	}

	// Export image as PNG using osascript
	tmpFile := filepath.Join(m.imagesDir, fmt.Sprintf("clip_%d.png", time.Now().UnixNano()))
	exportScript := fmt.Sprintf(`tell application "System Events"
	try
		set theData to the clipboard as «class PNGf»
		set f to open for access POSIX file "%s" with write permission
		set eof f to 0
		write theData to f
		close access f
		return "ok"
	on error
		return "fail"
	end try
end tell`, tmpFile)

	out, err = exec.Command("osascript", "-e", exportScript).Output()
	if err != nil || strings.TrimSpace(string(out)) != "ok" {
		_ = os.Remove(tmpFile)
		return false
	}

	// Check file size (max 5MB)
	info, err := os.Stat(tmpFile)
	if err != nil || info.Size() > 5*1024*1024 || info.Size() == 0 {
		_ = os.Remove(tmpFile)
		return false
	}

	// Dedup by hash of file content
	hash := hashFile(tmpFile)
	if hash == m.lastHash {
		_ = os.Remove(tmpFile)
		return true
	}
	m.lastHash = hash

	relPath := filepath.Join("data", "images", filepath.Base(tmpFile))
	m.store.Create("[图片]", "image", relPath)
	log.Printf("🖼️ Image captured: %s (%.1f KB)", filepath.Base(tmpFile), float64(info.Size())/1024)
	return true
}

func hashStr(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

func hashFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}
