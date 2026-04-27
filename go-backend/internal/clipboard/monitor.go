package clipboard

import (
	"log"
	"time"

	"github.com/atotto/clipboard"
)

// Monitor watches the system clipboard for changes
type Monitor struct {
	onChange func(content string)
	lastText string
	done     chan struct{}
}

// NewMonitor creates a new clipboard monitor
func NewMonitor(onChange func(string)) *Monitor {
	return &Monitor{
		onChange: onChange,
		done:     make(chan struct{}),
	}
}

// Start begins polling the clipboard
func (m *Monitor) Start() {
	go m.poll()
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	close(m.done)
}

func (m *Monitor) poll() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			text, err := clipboard.ReadAll()
			if err != nil {
				log.Printf("Clipboard read error: %v", err)
				continue
			}
			if text != "" && text != m.lastText {
				m.lastText = text
				m.onChange(text)
			}
		}
	}
}
