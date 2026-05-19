# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# termi

Go-based TUI SSH terminal manager with AI agent automation.

## Build

```bash
make deps      # install Go dependencies (requires Go 1.23+)
make build     # produces bin/termi
make run       # run without building binary
make test      # unit tests with race detector
make test-integration  # integration tests (build tag: integration)
make lint      # golangci-lint
make fuzz      # fuzz the VT parser for 60s
make install   # go install to $GOPATH/bin
```

Run a single test:

```bash
go test -run TestFooBar ./internal/store/
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

**Two-tier message packages**: `internal/msgs/msgs.go` is a zero-internal-import leaf package — all `tea.Msg` types that don't need higher-level types live here to break import cycles. `internal/app/messages.go` re-exports those and adds types that depend on `internal/ssh` and `internal/scheduler`. New messages belong in `msgs` unless they require an import that would create a cycle, in which case they go in `app/messages.go`.

- SQLite opens with WAL mode + `_foreign_keys=on`; use `DB.WithTx` for multi-statement operations
- SQLite migrations in `internal/store/migrations/` — numbered sequentially (`001_`, `002_`…), never modify existing files
- Credential secrets never hit the DB — use `internal/creds/` with OS keyring
- Audit every SSH command via `internal/audit/logger.go` — use `Log()` (non-blocking) not `LogSync()` in hot paths

## Configuration

Config file: `~/.config/termi/config.yaml` (all fields optional, sensible defaults apply).

```yaml
ssh:
  connect_timeout: 10s
  keepalive_interval: 30s

ai:
  ollama_host: http://localhost:11434
  ollama_model: llama3
  claude_model: claude-sonnet-4-6
  context_lines: 100      # lines of terminal output sent to AI
  openai_base_url: ""     # OpenAI-compatible endpoint (OpenWebUI, LiteLLM, vLLM…)
  openai_api_key: ""
  openai_model: gpt-4o

ui:
  escape_key: ctrl+b      # prefix key
  scrollback: 5000
```

## Environment variables

```
ANTHROPIC_API_KEY   — Claude AI panel
OLLAMA_HOST         — Ollama server (default http://localhost:11434)
OPENAI_BASE_URL     — OpenAI-compatible endpoint URL
OPENAI_API_KEY      — Bearer token for OpenAI-compatible endpoint
TERMI_DATA_DIR      — override ~/.local/share/termi
```
