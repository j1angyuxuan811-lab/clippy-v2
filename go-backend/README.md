# Clippy v2 - Go Backend

A clipboard manager backend written in Go for macOS that monitors the system clipboard and provides an HTTP API.

## Features

- Monitors system clipboard for text changes (polling every 500ms)
- Stores clipboard history in SQLite (max 1000 items, auto-cleanup)
- HTTP API on localhost:5100
- Deduplication of clipboard content
- Pinned items are preserved during cleanup

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/health` | Health check |
| GET | `/api/items?search=&limit=50` | List clipboard items |
| POST | `/api/items/:id/pin` | Toggle pin status |
| DELETE | `/api/items/:id` | Delete item |
| POST | `/api/paste/:id` | Get content for paste |

## Build & Run

```bash
# Build
CGO_ENABLED=1 go build -o clippy-backend .

# Run
./clippy-backend

# Or use Make
make run
```

## Command Line Options

- `-addr :5100` - HTTP server address (default: :5100)
- `-db clippy.db` - SQLite database path (default: clippy.db)

## Dependencies

- `github.com/mattn/go-sqlite3` - SQLite driver
- `github.com/atotto/clipboard` - Clipboard access
- `github.com/gorilla/mux` - HTTP router

## Database Schema

```sql
CREATE TABLE items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content TEXT NOT NULL,
    content_type TEXT DEFAULT 'text',
    pinned INTEGER DEFAULT 0,
    tags TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    access_count INTEGER DEFAULT 0
);
```

## Architecture

```
go-backend/
├── main.go                    # Entry point, orchestrates services
├── internal/
│   ├── clipboard/
│   │   └── monitor.go         # Clipboard polling logic
│   ├── db/
│   │   └── store.go           # SQLite operations
│   └── api/
│       └── server.go          # HTTP handlers
├── go.mod
├── go.sum
└── Makefile
```

## Notes

- Requires CGO_ENABLED=1 for go-sqlite3
- Designed for arm64 macOS
- Paste simulation returns content for Swift frontend to handle actual Cmd+V
