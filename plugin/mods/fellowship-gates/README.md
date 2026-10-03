# fellowship-gates (experimental Claude Code mod)

Shows fellowship quest status inside Claude Code:

- **Status line** — `⚔ 2 quests · 1 gate pending` (names the quest and phase when there is only one).
- **Toast** — when a gate becomes newly pending.
- **Gate band above the prompt** — one row per pending gate with **Approve**, **Reject…** and **Later**.
  - *Approve* submits "Approve the <phase> gate for quest …" as your prompt, so the lead runs `fellowship gate approve` *and* messages the teammate to resume.
  - *Reject…* pre-fills a rejection prompt for you to add feedback to.
  - *Later* hides that gate until its state changes.

Data comes from `~/.claude/fellowship/bin/fellowship status --json`, polled every 10 s and after each turn. With no binary or no quests it shows nothing.

## Try it

```sh
claude --plugin-dir plugin/mods/fellowship-gates
```

Then run `/fellowship` as usual.

## Develop

```sh
claude plugin validate plugin/mods/fellowship-gates
claude plugin test plugin/mods/fellowship-gates
```

Once Claude Code has loaded the mod it writes its API types to `.claude-plugin/types/` (gitignored), after which `tsc -p plugin/mods/fellowship-gates` type-checks it.
