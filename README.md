# termi

A terminal-based SSH session manager with Ansible automation and AI assistance.

termi lets you open multiple SSH sessions as tabs, broadcast commands across all of them at once, run Ansible playbooks, schedule unattended jobs, and ask Claude or Ollama to write the automation for you — all without leaving your terminal.

---

## Features

| Feature | Details |
|---|---|
| **SSH sessions** | Saved sessions with SSH agent, key file, key+passphrase (OS keyring), or password auth |
| **Terminal emulation** | Full VT100/xterm-256color inside a TUI — arrow keys, ctrl sequences, function keys all work |
| **Multi-tab** | Open as many sessions as you like, cycle with `ctrl+b tab` or jump with `ctrl+b 1–9` |
| **Broadcast** | Select any subset of open sessions and run one command across all of them simultaneously |
| **Ansible playbooks** | Register `.yml` playbooks, run them against any stored sessions — no active connection needed |
| **Scheduler** | Cron-based jobs that run Ansible playbooks in **interactive** (approval required) or **autonomous** (unattended + audited) mode |
| **Audit log** | Every command and playbook run is recorded with session, actor, exit code, and output snippet |
| **AI panel** | Chat with Claude, Ollama, or any OpenAI-compatible endpoint (OpenWebUI, LiteLLM, vLLM, Groq…); AI has context about your active sessions and recent terminal output; generates Ansible playbooks you can save to disk with one keystroke |
| **OS keyring** | Passwords and passphrases stored in GNOME Keyring / macOS Keychain / Windows Credential Manager — never in the database |

---

## Requirements

| Dependency | Required | Notes |
|---|---|---|
| Go 1.23+ | Yes | Build from source |
| SSH client | Yes | `ssh`/`openssh` on the local machine |
| `ansible-playbook` | For playbooks | `pip install ansible` or distro package |
| `sshpass` | Optional | Only needed for password-auth Ansible runs |
| Ollama | Optional | Local LLM; AI panel works without it if Claude is configured |
| `ANTHROPIC_API_KEY` | Optional | Enables Claude in the AI panel |
| `OPENAI_BASE_URL` | Optional | OpenAI-compatible endpoint (OpenWebUI, LiteLLM, vLLM, etc.) |
| `OPENAI_API_KEY` | Optional | Bearer token for the OpenAI-compatible endpoint |

---

## Install

### From source

```bash
git clone https://github.com/cyberslacks/termi
cd termi
go build -o bin/termi .
# or: go install .
```

### Makefile shortcuts

```bash
make build    # → bin/termi
make run      # run without building
make install  # go install to $GOPATH/bin
make test     # unit tests + race detector
make fuzz     # fuzz the VT parser for 60 seconds
```

---

## Quick start

### 1. Run termi

```bash
./bin/termi
# or after go install:
termi
```

### 2. Add your first session

Press `ctrl+b n` (or `ctrl+b s` then `n`) to open the new-session form.

```
Name           prod-web-01
Host           192.168.1.100
Port           22
User           ubuntu
Auth Method    Key File  (← → to cycle)
Key File Path  ~/.ssh/id_ed25519
```

Press `ctrl+s` to save.

### 3. Connect

From the session list, highlight your session and press `enter`. A terminal tab opens immediately and you are dropped into a live shell.

### 4. Open more sessions and broadcast

Connect to a second machine the same way. You now have two tabs.

Press `ctrl+b b` to open the broadcast screen:
- `space` — toggle selection on a session
- `a` — select / deselect all
- `enter` — proceed to the command prompt
- Type `uptime`, press `enter` — the command runs on every selected host in parallel and results appear per host.

---

## Keybindings

### Global prefix: `ctrl+b`

After pressing `ctrl+b`, press:

| Key | Action |
|---|---|
| `s` | Session list |
| `n` | New session (shortcut) |
| `p` | Playbooks |
| `S` | Scheduler |
| `a` | Audit log |
| `A` | AI panel |
| `b` | Broadcast |
| `tab` | Next terminal tab |
| `shift+tab` | Previous terminal tab |
| `1`–`9` | Jump to terminal tab N |
| `x` | Close active tab |
| `q` | Quit |

### Terminal tab (while connected)

| Key | Action |
|---|---|
| Normal typing | Forwarded to remote shell |
| `ctrl+b` | Enter prefix mode (not forwarded) |
| `pgup` / `pgdn` | Scroll through scrollback buffer |

### Session list

| Key | Action |
|---|---|
| `enter` | Connect |
| `n` | New session |
| `e` | Edit |
| `d` | Delete (with confirmation) |
| `esc` | Back |
| `/` | Filter (fuzzy search) |

### Broadcast screen

| Key | Action |
|---|---|
| `j` / `k` or `↑` / `↓` | Move cursor |
| `space` | Toggle session |
| `a` | Select / deselect all |
| `enter` | Confirm selection → type command |
| `esc` | Back |

After running:

| Key | Action |
|---|---|
| `j` / `k` | Navigate results |
| `enter` | Expand / collapse output |
| `r` | Re-broadcast same command |
| `esc` | Back |

### AI panel

| Key | Action |
|---|---|
| `enter` | Send message |
| `ctrl+p` | Save last YAML code block as a playbook file |
| `ctrl+b` | Swap backend (Claude ↔ Ollama) |
| `ctrl+k` | Clear conversation |
| `pgup` / `pgdn` | Scroll history |
| `esc` | Cancel stream / leave panel |

### Audit log

| Key | Action |
|---|---|
| `j` / `k` | Navigate rows |
| `enter` | Expand / collapse detail |
| `f` | Cycle filter (all → user → agent → scheduler) |
| `r` | Refresh |
| `g` / `G` | Jump to top / bottom |
| `esc` | Back |

---

## Authentication

termi supports four SSH auth methods, selectable per session:

| Method | Description |
|---|---|
| **SSH Agent** | Uses whatever keys are loaded in `ssh-agent`. Zero configuration. |
| **Key File** | Specify the path to a private key file. |
| **Key+Keyring** | Key file path + passphrase stored in the OS keyring. |
| **Password** | Password stored in the OS keyring via `go-keyring`. |

Credentials are **never stored in the SQLite database**. Only key file paths are stored; secrets go exclusively to the OS keyring.

For the OS keyring to work:

- **Linux**: `gnome-keyring` or `kwallet` must be running.
- **macOS**: Works out of the box via Keychain.
- **WSL**: Falls back to a file-based fallback in `~/.local/share/termi/`.

---

## Ansible playbooks

termi treats Ansible as its automation engine — no custom YAML format to learn.

### Register a playbook

Press `ctrl+b p` → `n`:

```
Name           install-nginx
Description    Install and start nginx
Playbook Path  ~/.local/share/termi/playbooks/install-nginx.yml
```

The `.yml` file is a standard Ansible playbook:

```yaml
---
- name: Install and start nginx
  hosts: all
  become: true
  tasks:
    - name: Install nginx
      ansible.builtin.package:
        name: nginx
        state: present

    - name: Ensure nginx is running
      ansible.builtin.service:
        name: nginx
        state: started
        enabled: true
```

### Run a playbook

Press `enter` or `r` on a playbook in the list. termi generates a temporary Ansible inventory from the sessions attached to your active tabs, then streams `ansible-playbook` output live.

**Key point**: you do not need an active SSH connection open in termi to run a playbook. Ansible makes its own connections using the credentials stored for each session.

### Use AI to write playbooks

1. Press `ctrl+b A` to open the AI panel.
2. Describe what you want: *"Write an Ansible playbook that installs Docker and adds the current user to the docker group"*.
3. The AI generates a proper Ansible playbook.
4. Press `ctrl+p` to save it to `~/.local/share/termi/playbooks/playbook-NNNN.yml`.
5. Register it in the playbook screen and run it.

---

## Scheduler

Schedule any registered playbook to run on a cron schedule.

Press `ctrl+b S` → `n`:

```
Name           nightly-updates
Cron           0 2 * * *
Playbook       system-updates
Session IDs    1,3,5
Mode           autonomous  (← → to toggle)
Enabled        yes
```

**Interactive mode** — at fire time, the TUI shows an approval prompt. You have 10 minutes to press `y` (approve) or `n` (reject). If you are not at the terminal, the job is skipped.

**Autonomous mode** — fires unattended. Every command is written to the audit log with actor `scheduler`. Review history later with `ctrl+b a`.

Cron expressions use the robfig/cron v3 format with optional seconds:

```
* * * * *          every minute
0 2 * * *          2 am daily
0 0 * * 0          midnight every Sunday
0 */6 * * *        every 6 hours
30 9 * * 1-5       9:30 am weekdays
```

---

## Configuration

termi looks for a config file at `~/.config/termi/config.yaml`. All settings have sensible defaults.

```yaml
# ~/.config/termi/config.yaml

ssh:
  connect_timeout: 10s
  keepalive_interval: 30s

ai:
  ollama_host: http://localhost:11434
  ollama_model: llama3
  claude_model: claude-sonnet-4-6
  context_lines: 100        # lines of terminal output sent to AI

ui:
  escape_key: ctrl+b        # prefix key
  scrollback: 5000          # lines of terminal scrollback
```

### Environment variables

| Variable | Effect |
|---|---|
| `ANTHROPIC_API_KEY` | Enable Claude in the AI panel |
| `OLLAMA_HOST` | Ollama server URL (overrides config) |
| `OPENAI_BASE_URL` | OpenAI-compatible endpoint URL |
| `OPENAI_API_KEY` | Bearer token for the OpenAI-compatible endpoint |
| `TERMI_DATA_DIR` | Override `~/.local/share/termi` data directory |

---

## Data directory

By default `~/.local/share/termi/`:

```
~/.local/share/termi/
├── termi.db                  # SQLite — sessions, playbooks, jobs, audit log
└── playbooks/                # Ansible .yml files saved from AI panel
```

Override with `--data-dir /path/to/dir` or `TERMI_DATA_DIR`.

---

## AI panel details

### Backend selection

Three backends are supported. Press `ctrl+b` inside the panel to cycle through whichever are configured.

| Backend | How to enable | Default model |
|---|---|---|
| **Claude** | Set `ANTHROPIC_API_KEY` | `claude-sonnet-4-6` |
| **OpenAI-compatible** | Set `OPENAI_BASE_URL` (and optionally `OPENAI_API_KEY`) | `gpt-4o` |
| **Ollama** | Run `ollama serve` locally | `llama3` |

The OpenAI-compatible backend works with any server that speaks the `/v1/chat/completions` streaming API: OpenWebUI, LiteLLM, LocalAI, vLLM, Groq, Together AI, and others.

```yaml
# ~/.config/termi/config.yaml
ai:
  openai_base_url: http://localhost:3000   # OpenWebUI default port
  openai_api_key: ""                        # leave blank if no auth required
  openai_model: llama3.2                    # any model loaded in OpenWebUI
```

Or via environment variables:

```bash
export OPENAI_BASE_URL=http://my-openwebui:3000
export OPENAI_API_KEY=sk-...          # optional
./bin/termi
```

### Context injected automatically

Every message includes:
- Names and hosts of all active SSH sessions
- Last 100 lines of visible output from the focused terminal tab (configurable via `ai.context_lines`)

This means you can paste an error, switch to the AI panel, and ask "what does this mean?" without copying anything.

### Saving generated playbooks

When the AI writes a playbook in a ```` ```yaml ```` block, press `ctrl+p` to write it to disk. You will see the path in the status bar. Then register it in the playbook screen (`ctrl+b p → n`) and point the path field at the saved file.

---

## Building from source

```bash
# Prerequisites: Go 1.23+
go version

# Clone and build
git clone https://github.com/cyberslacks/termi
cd termi
go build -o bin/termi .

# Run tests
go test ./... -race

# Fuzz the VT parser
go test -fuzz=FuzzVTParser ./pkg/vtparser/ -fuzztime=60s
```

No CGO required — uses `modernc.org/sqlite` (pure Go SQLite).

---

## Troubleshooting

**Terminal renders garbled characters**
Ensure your local terminal reports `TERM=xterm-256color` or `TERM=xterm`. termi requests `xterm-256color` from the remote PTY.

**OS keyring errors on Linux**
Start or unlock `gnome-keyring-daemon`: `gnome-keyring-daemon --start`. In headless environments, termi falls back to a local encrypted file.

**`ansible-playbook` not found**
Install Ansible: `pip install --user ansible` or `sudo apt install ansible`. The binary must be on `$PATH`.

**Password-auth Ansible runs fail**
Install `sshpass` (`sudo apt install sshpass`). Ansible requires it for password-based SSH.

**Ollama AI panel says "not configured"**
Ensure Ollama is running (`ollama serve`) and has a model pulled (`ollama pull llama3`). The default host is `http://localhost:11434`.

**Scheduler jobs don't fire**
Check that `Enabled` is set to `yes` and the cron expression is valid. termi uses robfig/cron v3 — test your expression at [crontab.guru](https://crontab.guru).
