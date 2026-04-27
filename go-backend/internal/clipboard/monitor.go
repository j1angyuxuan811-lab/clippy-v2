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
	store         *db.Store
	interval      time.Duration
	lastTextHash  string
	lastImageHash string
	imagesDir     string
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
	m.check()
	for {
		time.Sleep(m.interval)
		m.check()
	}
}

func (m *Monitor) check() {
	if m.checkImage() {
		return
	}
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
	if hash == m.lastTextHash {
		return
	}
	m.lastTextHash = hash
	m.lastImageHash = "" // reset image hash when text changes
	m.store.Create(text, "text", "")
}

func (m *Monitor) checkImage() bool {
	// Check clipboard info for image types
	infoOut, err := exec.Command("osascript", "-e", "clipboard info as text").Output()
	if err != nil {
		log.Printf("⚠️ clipboard info error: %v", err)
		return false
	}
	info := string(infoOut)
	log.Printf("🔍 clipboard info: %s", info[:min(60, len(info))])

	hasImage := strings.Contains(info, "PNGf") ||
		strings.Contains(info, "TIFF") ||
		strings.Contains(info, "JPEG") ||
		strings.Contains(info, "GIFf") ||
		strings.Contains(info, "8BPS")
	if !hasImage {
		log.Printf("🔍 no image in clipboard")
		return false
	}

	log.Printf("🔍 image detected, exporting...")

	// Export as PNG
	tmpFile := filepath.Join(m.imagesDir, fmt.Sprintf("clip_%d.png", time.Now().UnixNano()))
	exportScript := fmt.Sprintf(`set theData to the clipboard as «class PNGf»
set f to open for access POSIX file "%s" with write permission
set eof f to 0
write theData to f
close access f`, tmpFile)

	out, err := exec.Command("osascript", "-e", exportScript).CombinedOutput()
	log.Printf("🔍 export result: err=%v, out=%s", err, string(out))
	if err != nil || len(out) > 0 {
		// Fallback to TIFF
		_ = os.Remove(tmpFile)
		tmpFile = filepath.Join(m.imagesDir, fmt.Sprintf("clip_%d.tiff", time.Now().UnixNano()))
		exportScript = fmt.Sprintf(`set theData to the clipboard as TIFF picture
set f to open for access POSIX file "%s" with write permission
set eof f to 0
write theData to f
close access f`, tmpFile)
		out, err = exec.Command("osascript", "-e", exportScript).CombinedOutput()
		if err != nil || len(out) > 0 {
			_ = os.Remove(tmpFile)
			return false
		}
	}

	// Check file size (max 5MB)
	finfo, err := os.Stat(tmpFile)
	if err != nil || finfo.Size() > 5*1024*1024 || finfo.Size() == 0 {
		_ = os.Remove(tmpFile)
		return false
	}

	// Dedup by content hash — only save if different from last image
	hash := hashFile(tmpFile)
	if hash == m.lastImageHash {
		_ = os.Remove(tmpFile)
		return true
	}
	m.lastImageHash = hash
	m.lastTextHash = "" // reset text hash when image changes

	relPath := filepath.Join("data", "images", filepath.Base(tmpFile))
	m.store.Create("[图片]", "image", relPath)
	log.Printf("🖼️ Image captured: %s (%.1f KB)", filepath.Base(tmpFile), float64(finfo.Size())/1024)
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
