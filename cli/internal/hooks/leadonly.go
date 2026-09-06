package hooks

import "strings"

// Lead-only CLI commands, as seen from inside a quest worktree.
//
// `fellowship state ...` is the lead's own command set — it creates the
// fellowship, registers quests, and records which session is the lead. A
// teammate that can run one of those can name itself the lead
// (`state init --claim-lead`, or a plain `state init` that re-records the lead
// on the way past), after which worktree-guard treats it as the lead and locks
// the real one out. `fellowship init --phase/--plan-skip` moves a quest's phase,
// which is a gate decision.
//
// Neither is caught by the escape allowlist, which only decides what a *blocked*
// teammate may run: these were reaching the CLI whenever nothing was pending.

// LeadOnlyInvocation names the lead-only command found in a Bash command line.
type LeadOnlyInvocation struct {
	// Subcommand is "state" or "init".
	Subcommand string
	// Detail is the state subcommand ("init", "add-quest", ...) or, for init,
	// the phase it asks for.
	Detail string
}

// LeadOnlyCommands reports EVERY lead-only fellowship invocation in a Bash
// command line, in the order a shell would run them.
//
// All of them, not just the first: whether an invocation is actually a
// violation depends on the quest's current phase, which only the caller knows.
// Returning the first match let a harmless one mask the rest — a teammate that
// prefixed `fellowship init --phase <the phase it is already in>` (a no-op the
// caller waves through) got the `fellowship state init --claim-lead` chained
// behind it for free.
//
// The whole command is scanned, not just its first word, and the scan follows
// the shapes a shell actually runs: operators (`&&`, `||`, `;`, `|`, `&`,
// newlines) separate commands, command substitution (`$(...)`, backticks) and
// subshells run what is inside them — inside double quotes as well as outside —
// `sh -c "..."` and `eval "..."` carry a command inside an argument, and the
// binary may be named by any path. A command merely
// NAMED inside a quoted argument (`git commit -m "fellowship state init"`) is
// not an invocation and is left alone — that is why the scan tokenizes rather
// than matching on the raw string.
func LeadOnlyCommands(command string) []LeadOnlyInvocation {
	var found []LeadOnlyInvocation
	for _, tokens := range commandInvocations(command, 0) {
		for i := 0; i+1 < len(tokens); i++ {
			if !isFellowshipBinary(tokens[i]) {
				continue
			}
			switch tokens[i+1] {
			case "state":
				detail := ""
				if i+2 < len(tokens) {
					detail = tokens[i+2]
				}
				found = append(found, LeadOnlyInvocation{Subcommand: "state", Detail: detail})
			case "init":
				if phase, ok := initPhaseFrom(tokens[i+2:]); ok {
					found = append(found, LeadOnlyInvocation{Subcommand: "init", Detail: phase})
				}
			}
		}
	}
	return found
}

// LeadOnlyCommand reports the first lead-only fellowship invocation in a Bash
// command line, if any. Callers that decide per-invocation (gate-guard) must
// use LeadOnlyCommands instead — see why there.
func LeadOnlyCommand(command string) (LeadOnlyInvocation, bool) {
	found := LeadOnlyCommands(command)
	if len(found) == 0 {
		return LeadOnlyInvocation{}, false
	}
	return found[0], true
}

// InitPhaseRequest reports the phase a Bash command asks `fellowship init` to
// put the quest in, and whether the command asks for a phase at all. It is
// LeadOnlyCommands narrowed to the phase move — the check gate-guard makes
// against the quest's *current* phase, since re-running init for the phase the
// quest is already in moves nothing.
//
// It scans every invocation on the line, not just the first lead-only one: an
// earlier `fellowship state ...` on the same line is a different violation, and
// stopping at it would report "no phase asked for" while
// `fellowship state show && fellowship init --phase Implement` asks for one.
func InitPhaseRequest(command string) (string, bool) {
	for _, inv := range LeadOnlyCommands(command) {
		if inv.Subcommand == "init" {
			return inv.Detail, true
		}
	}
	return "", false
}

// initPhaseFrom reads the phase out of the arguments following `init`.
// --plan-skip implies Implement, exactly as runInit resolves it.
func initPhaseFrom(args []string) (string, bool) {
	phase, planSkip := "", false
	for j, arg := range args {
		switch {
		case arg == "--plan-skip" || arg == "-plan-skip":
			planSkip = true
		case arg == "--phase" || arg == "-phase":
			if j+1 < len(args) {
				phase = args[j+1]
			}
		case strings.HasPrefix(arg, "--phase="):
			phase = strings.TrimPrefix(arg, "--phase=")
		case strings.HasPrefix(arg, "-phase="):
			phase = strings.TrimPrefix(arg, "-phase=")
		}
	}
	if phase == "" && planSkip {
		phase = PlanSkipPhase
	}
	return phase, phase != ""
}

// maxShellNesting bounds the recursion into `sh -c` payloads. Two levels covers
// every real invocation; the limit is there so a crafted command cannot make
// the scan run away.
const maxShellNesting = 3

// commandInvocations breaks a Bash command line into the individual commands a
// shell would run, each as a token slice.
//
// Segments are split on the operators that separate commands, and a segment
// that runs a shell with an inline script (`sh -c "..."`, `bash -lc '...'`) or
// `eval` is followed into that script. Quoting is respected throughout: the
// script inside `sh -c` is one token to the outer split, and is only re-split
// because the token it belongs to names a shell.
func commandInvocations(command string, depth int) [][]string {
	var out [][]string
	for _, segment := range shellSegments(command) {
		tokens := shellFields(segment)
		if len(tokens) == 0 {
			continue
		}
		out = append(out, tokens)
		if depth >= maxShellNesting {
			continue
		}
		for _, nested := range nestedScripts(tokens) {
			out = append(out, commandInvocations(nested, depth+1)...)
		}
	}
	return out
}

// nestedScripts returns the inline scripts a token slice hands to another
// shell: the argument after a `-c` flag on a shell binary, and everything after
// `eval`.
func nestedScripts(tokens []string) []string {
	var scripts []string
	for i, tok := range tokens {
		switch {
		case isShellBinary(tok):
			for j := i + 1; j < len(tokens); j++ {
				if isDashCFlag(tokens[j]) && j+1 < len(tokens) {
					scripts = append(scripts, tokens[j+1])
					break
				}
			}
		case tok == "eval":
			if i+1 < len(tokens) {
				scripts = append(scripts, strings.Join(tokens[i+1:], " "))
			}
		}
	}
	return scripts
}

// isShellBinary reports whether a token names a shell that can be handed an
// inline script.
func isShellBinary(token string) bool {
	base := token
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	switch base {
	case "sh", "bash", "zsh", "dash", "ksh", "busybox":
		return true
	}
	return false
}

// isDashCFlag reports whether a token is the shell flag that introduces an
// inline script — "-c" and the combined forms ("-lc", "-lic", "-ec").
func isDashCFlag(token string) bool {
	if len(token) < 2 || token[0] != '-' || strings.HasPrefix(token, "--") {
		return false
	}
	return strings.ContainsRune(token[1:], 'c')
}

// shellSegments splits a command line into the pieces separated by the
// operators a shell treats as command boundaries, ignoring any that appear
// inside quotes. It is deliberately crude: the goal is that no invocation hides
// behind an operator, not that the result is runnable.
//
// Command substitution is a boundary too, and it is the one that does NOT stop
// at a double quote: `"$(fellowship state init)"` runs that command exactly as
// the bare form does, so `$(`, a backtick, and a bare subshell `(` all end a
// segment. A `(` that is neither preceded by `$` nor outside quotes is left
// alone — a conventional-commit scope (`git commit -m "fix(hooks): ..."`) is
// text, not a subshell — and a single-quoted run is literal all the way
// through, since no substitution happens inside one.
func shellSegments(command string) []string {
	var (
		segments []string
		cur      strings.Builder
		quote    rune
		prev     rune
	)
	flush := func() {
		if strings.TrimSpace(cur.String()) != "" {
			segments = append(segments, cur.String())
		}
		cur.Reset()
	}
	for _, r := range command {
		escaped := prev == '\\'
		prev = r
		if escaped {
			// A backslash-escaped character is literal, and cannot itself
			// escape the character after it.
			cur.WriteRune(r)
			prev = 0
			continue
		}
		switch {
		case quote == '\'':
			cur.WriteRune(r)
			if r == '\'' {
				quote = 0
			}
		case r == '`':
			flush()
		case r == '(' && (quote == 0 || strings.HasSuffix(cur.String(), "$")):
			flush()
		case r == ')' && quote == 0:
			flush()
		case quote != 0:
			cur.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
			cur.WriteRune(r)
		case r == ';' || r == '&' || r == '|' || r == '\n' || r == '\r':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return segments
}

// HasCompleteCommand reports whether a Bash command line runs
// `fellowship complete` anywhere — through operators, substitutions, `sh -c`
// and `eval`, exactly as LeadOnlyCommands scans. gate-guard judges such a
// line against CompletionCheck so the quest cannot be ended before Review by
// shelling past the command's own refusal.
func HasCompleteCommand(command string) bool {
	for _, tokens := range commandInvocations(command, 0) {
		for i := 0; i+1 < len(tokens); i++ {
			if isFellowshipBinary(tokens[i]) && tokens[i+1] == "complete" {
				return true
			}
		}
	}
	return false
}
