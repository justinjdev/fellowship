# Conflict Resolution Protocol

When Palantir raises a file conflict alert (two quests modifying the same file), Gandalf follows this protocol:

## 1. Pause

Hold the later quest immediately to prevent further divergence:

```bash
fellowship hold --dir <later-quest-worktree> --reason "file conflict with <other-quest>: <file_path>"
```

This structurally blocks the held quest's Edit/Write/Bash/Agent/Skill tools via the gate-guard hook. The quest cannot proceed until unheld.

Notify the held quest via SendMessage explaining the hold.

## 2. Assess

Read both quests' plans and diffs to determine the conflict type:

- **Real conflict**: Both quests modify the same function, section, or logical unit. Merging will require manual resolution.
- **Incidental overlap**: Both quests touch the same file but different, non-overlapping sections. Merging is straightforward.

## 3. Resolve

Pick one of three strategies based on the assessment:

**Sequence** (for real conflicts): Let the earlier quest finish first. After it merges, rebase the held quest's worktree onto the updated main branch, then unhold. This is the safest strategy — no concurrent modifications to the same code.

**Partition** (for real conflicts that can be separated): Assign non-overlapping regions of the file to each quest. Update both quests' plans via SendMessage to clarify boundaries, then unhold. Only use this when the conflict is in clearly separable sections.

**Merge** (for incidental overlaps): Let both quests proceed. The later quest to merge handles any trivial conflicts during its PR. Unhold immediately with instructions to be aware of the overlap.

## 4. Resume

Unhold the paused quest:

```bash
fellowship unhold --dir <quest-worktree>
```

Send a message to the resumed quest via SendMessage with:
- The resolution strategy chosen
- Any updated instructions (e.g., "avoid modifying lines 50-80 in auth.go — quest-1 owns that section")
- Context about what the other quest is doing in the shared file

## Hold Outside Conflicts

Hold/unhold can also be used outside the conflict protocol — for example, to pause a quest while waiting for a user decision or external dependency. The mechanism is general-purpose.
