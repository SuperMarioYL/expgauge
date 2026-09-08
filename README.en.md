**English** | [简体中文](./README.md)

<div align="center">

<img src="https://readme-typing-svg.demolab.com?font=Fira+Code&weight=600&size=28&duration=3000&pause=1000&color=7D3CFF&center=true&vCenter=true&width=600&height=100&lines=expgauge;run-level+exposure+gauge;for+coding+agents" alt="expgauge" />

<br/>

<a href="https://github.com/SuperMarioYL/expgauge"><img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go" /></a>
<a href="https://github.com/SuperMarioYL/expgauge/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-7D3CFF?style=flat-square" alt="License" /></a>
<a href="https://github.com/SuperMarioYL/expgauge"><img src="https://img.shields.io/github/stars/SuperMarioYL/expgauge?style=flat-square&color=7D3CFF&logo=github" alt="Stars" /></a>
<a href="https://github.com/SuperMarioYL/expgauge/releases"><img src="https://img.shields.io/badge/version-0.1.0-7D3CFF?style=flat-square" alt="Version" /></a>
<a href="https://gitee.com/SuperMarioYL/expgauge"><img src="https://img.shields.io/badge/Gitee-mirror-C71D23?style=flat-square&logo=gitee&logoColor=white" alt="Gitee" /></a>

<br/>
<br/>

**Your Qwen agent has been running for 6 hours, made 47 file edits, and burned 2.3M tokens since you last looked.**

expgauge is a run-level exposure gauge for coding agents on local models. It tracks cumulative unsupervised exposure (wall-clock time, state mutations, token burn) and forces a human review checkpoint before things go sideways.

Free · Open Source · MIT Licensed

</div>

---

## Table of Contents

- [Why expgauge](#why-expgauge)
- [Quickstart](#quickstart)
- [Commands](#commands)
- [Live Demo](#live-demo)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [Roadmap](#roadmap)

---

## Why expgauge

Developers running coding agents on local models like Qwen3 and DeepSeek increasingly leave them unattended for 8+ hours at a stretch. The community celebrates "8 hours non-stop" as a breakthrough — but the only safety mechanism is your own trust. No gauge tells you how long the agent has run since your last review, how many files it mutated, or how many tokens it burned since you last looked.

DeepSeek-Reasonix (35k stars) markets "leave it running" as a feature, yet provides no run-level safety telemetry. The "I Have Been Clawed" incident index proves that unattended agents do go wrong — but it catalogs incidents after the damage, not before.

expgauge fills that gap:

| Dimension | What it tracks | Default threshold |
|-----------|---------------|-------------------|
| Wall-clock | Time since last human review checkpoint | 2 hours |
| Mutations | File write / create / delete events (fsnotify) | 50 |
| Tokens | Token usage parsed from agent logs | 1 million |

When any threshold is crossed, the terminal pauses with a review prompt — you must manually approve (y/n) before the agent can continue.

This is not per-request observability. This is not per-action risk scoring. This is **run-level cumulative exposure tracking with a forced review checkpoint**.

---

## Quickstart

### Install

```bash
go install github.com/SuperMarioYL/expgauge@latest
```

Or build from source:

```bash
git clone https://github.com/SuperMarioYL/expgauge.git
cd expgauge
go build -o expgauge ./cmd/expgauge
```

### First run

```bash
# Monitor file mutations in the current directory, track wall-clock exposure
expgauge watch --paths .

# Specify agent PID + log path to also track token burn
expgauge watch --pid $(pgrep -f coding-agent) --log /tmp/agent.log --paths ./src

# Check current run status
expgauge status

# Mark a human review checkpoint (resets wall-clock timer)
expgauge review

# Export the run timeline as JSONL
expgauge export -o timeline.jsonl
```

Works with zero config — built-in default thresholds (2h / 50 mutations / 1M tokens). Customize via [configuration](#configuration).

---

## Commands

| Command | Description |
|---------|-------------|
| `expgauge watch` | Start monitoring and show a live exposure badge; pauses at thresholds |
| `expgauge status` | Show the current run's cumulative exposure |
| `expgauge review` | Mark a human review checkpoint, resetting the wall-clock timer |
| `expgauge export -o file.jsonl` | Export the run timeline as JSONL |

`watch` command flags:

| Flag | Description |
|------|-------------|
| `--pid, -p` | Agent process PID (overrides config) |
| `--paths, -w` | Directories to watch for mutations |
| `--log, -l` | Agent log file path for token parsing |

---

## Live Demo

When `expgauge watch` starts, the terminal shows a live exposure badge:

```
 expgauge  exposure badge

  status: OK

  wall 0m05s  ██░░░░░░░░░░░░░░░░░░  0%
  mut  3       ██████░░░░░░░░░░░░░░  6%
  tok  12.5K   █░░░░░░░░░░░░░░░░░░░  1%

  [q] quit
```

When a threshold is crossed, the terminal pauses with a review prompt:

```
 ⚠ THRESHOLD EXCEEDED

The agent has crossed an exposure threshold:

  wall-clock:  2h01m
  mutations:   52
  tokens:      1.2M

  Review the agent's work. Continue? [y/n]
```

Press `y` to continue (resets wall-clock), `n` to stop monitoring.

---

## Configuration

The config file lives at `~/.expgauge.yaml`. All fields are optional — missing fields use built-in defaults:

```yaml
# Directories to watch for file mutations
watch_paths:
  - /home/user/my-project

thresholds:
  wall_clock_sec: 7200   # 2 hours
  mutations: 50           # 50 file mutations
  tokens: 1000000         # 1M tokens

agent:
  pid_file: /tmp/agent.pid         # Read agent PID from this file
  log_path: /tmp/agent.log        # Parse token usage from this log
  name_pattern: "qwen|deepseek|glm" # Search by process name with pgrep -f
```

`expgauge watch` locates the agent process in this order: `--pid` flag > `agent.pid_file` > `agent.name_pattern`. If none is configured, only filesystem mutations are tracked (no token tracking).

---

## Architecture

```
expgauge
├── cmd/expgauge/          # cobra CLI entrypoint (watch / status / review / export)
├── internal/
│   ├── config/            # ~/.expgauge.yaml parsing + defaults
│   ├── watcher/           # Agent process detection (PID file / pgrep / explicit PID)
│   ├── metrics/           # Exposure accumulator (wall-clock + fsnotify + log tail)
│   ├── threshold/         # Threshold evaluation + SIGTSTP force-pause
│   ├── ui/                 # bubbletea/lipgloss live badge TUI
│   └── export/            # JSONL timeline export
```

Data flow:

1. `watch` loads config → locates agent process (optional) → creates accumulator (fsnotify + log tail)
2. Accumulator polls filesystem events and log tail in background, updating wall-clock / mutations / tokens
3. TUI badge refreshes every second, evaluating thresholds
4. Threshold crossed → SIGTSTP sent to agent process + review prompt displayed
5. Run state persisted to `~/.expgauge/run.json` every minute, snapshots appended to `~/.expgauge/timeline.jsonl`
6. `status` / `review` / `export` read and write state and timeline files

---

## Roadmap

| Version | Goal | Status |
|---------|------|--------|
| v0.1 | Core: CLI watch + status + threshold pause + JSONL export | ✅ Implemented |
| v0.2 | Enhanced config, per-project thresholds, web dashboard | Planned |
| v0.3 | Multi-agent orchestration, cloud-hosted metrics | Planned |

---

<div align="center">

MIT © 2026 SuperMarioYL

</div>
