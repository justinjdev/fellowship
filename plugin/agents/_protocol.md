# Fellowship Agent Messaging Protocol

Canonical spec for how fellowship agents deliver reports and respond to lifecycle events via `SendMessage`.

> **Sync note:** Agent definition files must be self-contained at runtime — an installed plugin's agents cannot reliably read sibling files (their working directory is the user's project, not the plugin cache). Each agent therefore embeds the parts of this protocol it needs inline. When you edit this spec, update the embedded copies in `balrog.md`, `palantir.md`, and `scout.md`.

## Sending a Report

Use `SendMessage` to deliver findings or status to the requesting agent:

```json
{
  "type": "message",
  "recipient": "<requester>",
  "content": "[markdown content]",
  "summary": "[short one-line summary for logs]"
}
```

- `recipient`: whoever requested the work, as provided at spawn time:
  - Agents spawned by a teammate (e.g. balrog, spawned by a quest runner): the **requester task ID** from the spawn context.
  - Agents spawned by the fellowship lead (e.g. palantir): the lead — `"team-lead"`.
  - If no requester was provided (standalone mode), present output directly to the user instead of using SendMessage.
- `content`: full markdown report body.
- `summary`: one-line description shown in task logs (e.g., `"balrog: 2 critical, 1 high, 0 medium, 3 low findings"`).

## Shutdown

When you receive a shutdown request via `SendMessage`, respond immediately and stop:

```json
{
  "type": "shutdown_response",
  "request_id": "<from the incoming message>",
  "approve": true
}
```

Do not perform any further work after sending a shutdown response.
