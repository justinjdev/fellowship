# Fellowship Agent Messaging Protocol

Canonical spec for how fellowship agents deliver reports and respond to lifecycle events via `SendMessage`.

> **Sync note:** Agent definition files must be self-contained at runtime — an agent's markdown is injected as its system prompt with no path context, so an installed plugin's agents cannot reliably read sibling files (their working directory is the user's project, not the plugin cache). Skills are different: the Skill tool exposes the invoked skill's directory, which is why SKILL.md files can reference sibling `resources/*.md` while agents cannot. Each agent therefore embeds the parts of this protocol it needs inline. When you edit this spec, update the embedded copies in `balrog.md`, `palantir.md`, and `scout.md`.

## Sending a Report

Use `SendMessage` to deliver findings or status to the requesting agent:

```
SendMessage(
  to: "<requester>",
  summary: "<short one-line summary for logs>",
  message: "<first line is the envelope header>\n<full markdown report body>"
)
```

- `to`: whoever requested the work, as provided at spawn time. **SendMessage addresses agents by name, not task ID.**
  - Agents spawned by a teammate (e.g. balrog, spawned by a quest runner): the **requester's teammate name** from the spawn context (e.g. `"quest-auth-bug"`).
  - Agents spawned by the fellowship lead (e.g. palantir): the lead — `"main"`.
  - If no requester name was provided (standalone mode), present output directly to the user instead of using SendMessage.
- `message`: first line is the envelope header (the recipient sees only this first line as the preview, so it must be self-contained), followed by the full markdown report body.
- `summary`: one-line description shown in the sender's own transcript (e.g., `"balrog: 2 critical, 1 high, 0 medium, 3 low findings"`).

## Stopping

The lead stops you with `TaskStop` when your work is cancelled or the fellowship disbands; there is nothing to respond to. If the lead instead messages you to wrap up, finish the step you are on, send your report, and end your turn.
