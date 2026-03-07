# Fellowship Dashboard Design

## Overview

A live web dashboard for monitoring and interacting with active fellowships. Runs as a local HTTP server via `fellowship dashboard`, serving a single-page UI with vanilla JS polling and gate approve/reject actions.

## Architecture

Single binary, everything embedded via Go `embed.FS`. New `dashboard` subcommand on the existing CLI.

```
fellowship dashboard [--port 3000] [--poll 5]
```

On start:
1. Resolve git root directory
2. Load fellowship state (primary: `tmp/fellowship-state.json`, fallback: `git worktree list` + probe for `tmp/quest-state.json`)
3. Start HTTP server on localhost
4. Open browser via `open` (macOS) / `xdg-open` (Linux)
5. Print URL to stdout

### Embedded Assets

All served from Go `embed.FS` — no external files needed:
- `dashboard/index.html` — single page
- `dashboard/style.css` — Tolkien-minimal styling
- `dashboard/app.js` — vanilla JS, polling + rendering

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/status` | Full fellowship state (all quests, phases, gates) |
| `POST` | `/api/gate/approve` | Approve a pending gate `{"dir": "/path/to/worktree"}` |
| `POST` | `/api/gate/reject` | Reject a pending gate `{"dir": "/path/to/worktree"}` |

Static assets served at `/` (index.html, style.css, app.js).

Approve/reject endpoints reuse existing `state.Load()` / `state.Save()` / `state.NextPhase()` logic from `cli/internal/state/`.

## Data Model

### Fellowship-Level State File

New file: `tmp/fellowship-state.json` (written by Gandalf at fellowship startup).

```json
{
  "name": "auth-overhaul",
  "created_at": "2026-03-06T10:30:00Z",
  "quests": [
    {"name": "quest-api-auth", "worktree": "/path/to/worktree", "task_id": "task-123"},
    {"name": "quest-db-schema", "worktree": "/path/to/worktree", "task_id": "task-456"}
  ],
  "scouts": [
    {"name": "scout-oauth", "task_id": "task-789"}
  ]
}
```

### Status API Response

```json
{
  "name": "auth-overhaul",
  "quests": [
    {
      "name": "quest-api-auth",
      "worktree": "/path/to/worktree",
      "phase": "Implement",
      "gate_pending": false,
      "gate_id": null,
      "lembas_completed": true,
      "metadata_updated": true
    }
  ],
  "scouts": [
    {"name": "scout-oauth", "task_id": "task-789"}
  ],
  "poll_interval": 5
}
```

Built by reading `tmp/fellowship-state.json` for quest/scout list, then loading each quest's `tmp/quest-state.json` from its worktree for live state. Falls back to `git worktree list` scanning if no fellowship state file exists.

## UI Design

Single page, three zones.

### Zone 1 — Header

Fellowship name, quest/scout counts, poll interval indicator with subtle pulse animation.

### Zone 2 — Quest Cards

One card per quest/scout:
- Name, current phase, progress bar (phases 1-6)
- Gate status: hidden when not pending, shows approve/reject buttons when pending
- Pending gates highlighted with warm amber glow
- Scout cards simplified — name and "research in progress" only, no gate actions

### Zone 3 — Activity Feed

Recent state changes, kept client-side only. Appended on each poll when data changes. Clears on page refresh. No server-side persistence.

### Aesthetic: Tolkien-Minimal

- Dark background with warm, muted earth tones (deep brown, aged gold, forest green)
- Medieval-inspired serif for headings (Cinzel), clean sans-serif for body
- Phase progress bar in gold/amber
- Pending gates with warm amber glow
- Subtle knotwork border on header, minimal ornamentation elsewhere
- Cards with slight parchment warmth, no full texture

## Gate Actions

1. User clicks Approve or Reject on a pending gate card
2. JS sends POST to `/api/gate/approve` or `/api/gate/reject` with `{"dir": "/path/to/worktree"}`
3. Server loads quest state, calls existing state logic
4. Returns updated quest JSON; client updates card immediately
5. Reject shows inline "Are you sure?" confirmation; Approve fires immediately

Error handling: missing state file or no pending gate returns 400 with message, shown as toast on the card.

## Configuration

- `--port`: HTTP port (default: 3000)
- `--poll`: Polling interval in seconds (default: 5, configurable via query param `?poll=N`)

## Discovery Strategy

Dual-source quest discovery:
1. **Primary**: `tmp/fellowship-state.json` — authoritative list of quests and worktree paths
2. **Fallback**: `git worktree list` — scan each worktree for `tmp/quest-state.json`, filter to fellowship worktrees

## Prerequisites

- Fellowship skill must write `tmp/fellowship-state.json` at startup (shared prerequisite with #14, #16, #18)
- Dashboard works without it via worktree scanning fallback

## Scope

- Go: ~400 lines (HTTP server, endpoints, quest discovery)
- HTML/CSS/JS: ~300 lines (single page, cards, polling, gate actions)
- Fellowship skill: ~10 lines (write `tmp/fellowship-state.json`)
