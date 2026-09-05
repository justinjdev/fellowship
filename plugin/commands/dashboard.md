---
description: Starts the live web dashboard for the current fellowship — quest/scout progress, gate approvals, and event history — in the background, and prints the URL.
---

# Dashboard — Live Fellowship Web UI

## Overview

Starts a local HTTP server that shows the current fellowship at a glance: every quest's phase and pending gates, scouts, companies, the tiding (event) stream, the bulletin board, and eagles health. Gates can be approved or rejected directly from the page, including batch-approving every pending gate in a company.

## Steps

### Step 1: Start the Server

Run the binary in the background so it doesn't block the session, redirecting output to a log file:

```bash
nohup ~/.claude/fellowship/bin/fellowship dashboard > /tmp/fellowship-dashboard.log 2>&1 &
disown
```

Defaults to port 3000 and a 5-second poll interval. If the user wants a different port or poll interval, pass flags:

```bash
nohup ~/.claude/fellowship/bin/fellowship dashboard --port 3001 --poll 10 > /tmp/fellowship-dashboard.log 2>&1 &
disown
```

If port 3000 (or the requested port) is already in use — for example a dashboard from a previous session is still running — check `/tmp/fellowship-dashboard.log` and either reuse the existing dashboard's URL or pick a free port with `--port`.

### Step 2: Report the URL

Print the URL (`http://localhost:<port>`, `3000` unless overridden) and briefly explain what it shows:

> **Fellowship dashboard:** http://localhost:3000
>
> Live view of every quest's phase and pending gates, scouts, companies (with one-click "Approve All"), the recent event stream, the bulletin board, and eagles health. Approvals and rejections made on the page take effect immediately — no need to poll `fellowship status` yourself.

The server keeps running after this command finishes; the user can leave the tab open across the rest of the session.
