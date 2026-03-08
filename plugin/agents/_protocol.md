# Fellowship Agent Messaging Protocol

Shared protocol for all fellowship agents. Agents that communicate via `SendMessage` and respond to lifecycle events must follow this spec.

## Sending a Report

Use `SendMessage` to deliver findings or status to the requesting agent:

```json
{
  "type": "message",
  "recipient": "<requester_task_id>",
  "content": "[markdown content]",
  "summary": "[short one-line summary for logs]"
}
```

- `recipient`: the task ID provided at spawn time. If not provided (standalone mode), present output directly to the user instead.
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
