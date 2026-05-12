# Getting Started with termi

This guide walks you from zero to having a working setup: a session connected, a broadcast run, a playbook written by AI, and a scheduled job.

---

## Step 1 — Install

```bash
git clone https://github.com/cyberslacks/termi
cd termi
go build -o bin/termi .
```

Confirm it works:

```bash
./bin/termi --help
```

To install system-wide:

```bash
go install .
# then: termi
```

---

## Step 2 — Launch

```bash
./bin/termi
```

You land on the dashboard. Everything is driven by a **prefix key** — by default `ctrl+b` — followed by a letter.

```
ctrl+b s  — sessions
ctrl+b n  — new session
ctrl+b p  — playbooks
ctrl+b S  — scheduler
ctrl+b a  — audit log
ctrl+b A  — AI panel
ctrl+b b  — broadcast
ctrl+b q  — quit
```

---

## Step 3 — Add a session

Press `ctrl+b n`. Fill in the form (Tab moves between fields):

```
Name      my-server
Host      192.168.1.100
Port      22
User      ubuntu
```

For **Auth Method**, use `←` / `→` to cycle through options:

| Option | When to use |
|---|---|
| SSH Agent | You have keys loaded in `ssh-agent` (simplest) |
| Key File | Private key on disk, no passphrase |
| Key+Keyring | Private key on disk with passphrase stored in OS keyring |
| Password | Password stored in OS keyring |

Press `ctrl+s` to save. The session list reloads and your server appears.

---

## Step 4 — Connect

Highlight your session in the list and press `enter`. A terminal tab opens and your shell prompt appears. Type normally — all keystrokes go directly to the remote shell.

```bash
# Anything you type here is live on the remote machine
whoami
df -h
```

Press `ctrl+b` to enter prefix mode (that one keystroke is captured by termi, not forwarded). Then:
- `ctrl+b tab` — open session list to connect another host
- `ctrl+b b` — broadcast screen
- `ctrl+b a` — audit log

**Scroll the terminal**: `pgup` / `pgdn` (up to 5,000 lines of scrollback).

---

## Step 5 — Add a second session and broadcast

Connect to a second machine the same way (`ctrl+b n` or `ctrl+b s → n`). Once connected, you have two terminal tabs.

Press `ctrl+b b` for the broadcast screen. You will see both sessions listed.

```
  [ ] my-server    192.168.1.100   connected 2m
  [ ] my-server-2  192.168.1.101   connected 30s
```

- `space` — toggle a session
- `a` — select all
- `enter` — proceed to command input

Type your command and press `enter`:

```
$ free -h
```

Results appear per host showing stdout, exit code, and whether it succeeded. Press `enter` on any row to expand the full output. Press `r` to send another command to the same selection.

---

## Step 6 — AI panel and Ansible playbooks

### Set up the AI backend

Three backends are available. Configure one or more — press `ctrl+b` inside the panel to cycle between them.

**Option A — Ollama (local, free)**

```bash
# Install Ollama: https://ollama.com
ollama serve            # start the server
ollama pull llama3      # pull a model
```

termi detects Ollama automatically at `http://localhost:11434`.

**Option B — Claude**

```bash
export ANTHROPIC_API_KEY=sk-ant-...
./bin/termi
```

**Option C — OpenAI-compatible endpoint (OpenWebUI, LiteLLM, vLLM, Groq, etc.)**

```bash
export OPENAI_BASE_URL=http://localhost:3000   # your OpenWebUI URL
export OPENAI_API_KEY=sk-...                   # omit if no auth required
./bin/termi
```

Or add to `~/.config/termi/config.yaml`:

```yaml
ai:
  openai_base_url: http://localhost:3000
  openai_model: llama3.2    # any model available in your endpoint
```

Any server that speaks the standard `/v1/chat/completions` streaming API works.

### Ask the AI to write a playbook

Press `ctrl+b A` to open the AI panel. Type:

```
Write an Ansible playbook that installs nginx, enables it on boot, and 
ensures port 80 is open in ufw.
```

The AI responds with an explanation followed by a ```` ```yaml ```` block. Press `ctrl+p` to save it to disk — you will see the file path in the status bar at the bottom.

### Register and run the playbook

1. Press `ctrl+b p` to open the playbook screen.
2. Press `n` to add a new entry.
3. Fill in **Name**, **Description**, and paste the file path shown in the status bar.
4. Press `ctrl+s` to save.
5. Highlight the playbook and press `enter` or `r` to run it.

termi generates a temporary Ansible inventory from your stored sessions and streams `ansible-playbook` output live. You will see task results as they complete.

> **Tip**: you do not need to have an SSH tab open to run a playbook. Ansible makes its own connections using the credentials you stored for each session.

---

## Step 7 — Schedule a playbook

Press `ctrl+b S` to open the scheduler, then `n` for a new job.

```
Name           weekly-updates
Cron           0 2 * * 0        (2 am every Sunday)
Playbook       system-updates   (← → to cycle through registered playbooks)
Session IDs    1,2              (comma-separated IDs from the session list)
Mode           interactive      (← → to toggle)
Enabled        yes
```

Press `ctrl+s` to save. The job is registered immediately.

**Interactive mode**: when the job fires, the status bar shows an approval prompt. Press `y` to run it, `n` to cancel. If you are not at the terminal, the job is skipped.

**Autonomous mode**: the job runs without intervention. All output is written to the audit log.

To switch an existing job between modes, highlight it and press `e` to edit.

---

## Step 8 — Review the audit log

Press `ctrl+b a`. Every SSH command and every Ansible run appears here:

```
  Time             Session       Actor    Command                      Exit
  05-11 02:00:01   my-server     sched    ansible-playbook update...   0
  05-11 09:14:33   my-server-2   user     df -h                        0
```

Navigate with `j` / `k`. Press `enter` to expand a row and see the full output snippet.

Press `f` to cycle through filters: **all → user → agent → scheduler**.

---

## Day-to-day workflow

```
termi
  └─ ctrl+b s       open sessions, connect one or more
  └─ ctrl+b b       broadcast a one-off command to all of them
  └─ ctrl+b A       ask AI to write automation
     └─ ctrl+p      save the YAML to disk
  └─ ctrl+b p       register the playbook and run it
  └─ ctrl+b S       schedule it for later
  └─ ctrl+b a       check what ran and whether it succeeded
```

---

## Configuration reference

Create `~/.config/termi/config.yaml` to override defaults:

```yaml
ssh:
  connect_timeout: 10s
  keepalive_interval: 30s

ai:
  ollama_host: http://localhost:11434
  ollama_model: llama3
  claude_model: claude-sonnet-4-6
  context_lines: 100

ui:
  escape_key: ctrl+b
  scrollback: 5000
```

Environment variables override the config file:

| Variable | Effect |
|---|---|
| `ANTHROPIC_API_KEY` | Enable Claude |
| `OLLAMA_HOST` | Ollama server URL |
| `OPENAI_BASE_URL` | OpenAI-compatible endpoint (OpenWebUI, LiteLLM, etc.) |
| `OPENAI_API_KEY` | Bearer token for that endpoint |
| `TERMI_DATA_DIR` | Override `~/.local/share/termi` |

---

## Quick reference card

```
PREFIX = ctrl+b

Navigation
  PREFIX s      sessions list
  PREFIX n      new session
  PREFIX p      playbooks
  PREFIX S      scheduler
  PREFIX a      audit log
  PREFIX A      AI panel
  PREFIX b      broadcast
  PREFIX tab    next tab
  PREFIX 1-9    jump to tab N
  PREFIX x      close tab
  PREFIX q      quit

In terminal tab
  pgup/pgdn     scroll history

Session list
  enter         connect
  e             edit
  d             delete
  /             fuzzy search

Broadcast
  space         toggle host
  a             all / none
  enter         next step

AI panel
  enter         send
  ctrl+p        save playbook to disk
  ctrl+b        swap Claude ↔ Ollama
  ctrl+k        clear history

Scheduler list
  n             new job
  e             edit
  d             delete
  t             enable / disable

Audit log
  j/k           navigate
  enter         expand row
  f             cycle filter
  r             refresh
```
