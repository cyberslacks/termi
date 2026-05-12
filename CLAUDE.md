# termi

Go-based TUI SSH terminal manager with AI agent automation.

## Build

```bash
make deps      # install Go dependencies (requires Go 1.22+)
make build     # produces bin/termi
make run       # run without building binary
make test      # unit tests with race detector
make fuzz      # fuzz the VT parser for 60s
```

## Architecture

**Entry point**: `main.go` → `cmd/root.go` → `internal/app/app.go`

**Critical paths**:
- SSH PTY output: `internal/ssh/session.go` StdoutPipe → `internal/ui/terminal/pty_bridge.go` goroutine → `app.TermOutputMsg` → `internal/ui/terminal/model.go` Update → `pkg/vtparser/screen.go` Write → `internal/ui/terminal/view.go`
- Playbook execution: `internal/playbook/executor.go` → `internal/ssh/manager.go` RunCommand → `internal/audit/logger.go`
- Scheduler: `internal/scheduler/runner.go` (robfig/cron) → approval channel → `internal/app/app.go`

**VT parser** (`pkg/vtparser/`): cell-grid framebuffer rendered by lipgloss. Fuzz-test before modifying.

**Bubbletea model** (`internal/app/app.go`): `RootModel` owns all tabs + services. PTY output messages flow at ~60fps per active session. Keep `Update()` fast.

## Key conventions

- All `tea.Msg` types are in `internal/app/messages.go` — add new ones there only
- SQLite migrations in `internal/store/migrations/` — numbered sequentially, never modify existing
- Credential secrets never hit the DB — use `internal/creds/` with OS keyring
- Audit every SSH command via `internal/audit/logger.go` — use `Log()` (non-blocking) not `LogSync()` in hot paths

## Environment variables

```
ANTHROPIC_API_KEY   — Claude AI panel
OLLAMA_HOST         — Ollama server (default http://localhost:11434)
TERMI_DATA_DIR      — override ~/.local/share/termi
```
