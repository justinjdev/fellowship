package hooks

import "testing"

func TestBootstrapGuard(t *testing.T) {
	cases := []struct {
		name  string
		input *HookInput
		block bool
	}{
		{"source write", &HookInput{ToolName: "Write", ToolInput: ToolInput{FilePath: "/wt/calc.py"}}, true},
		{"source edit", &HookInput{ToolName: "Edit", ToolInput: ToolInput{FilePath: "/wt/src/a.go"}}, true},
		{"data dir write (plan copy)", &HookInput{ToolName: "Write", ToolInput: ToolInput{FilePath: "/wt/.fellowship/plan.md"}}, false},
		{"store write", &HookInput{ToolName: "Write", ToolInput: ToolInput{FilePath: "/wt/.fellowship/fellowship.db"}}, true},
		{"fellowship init", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "~/.claude/fellowship/bin/fellowship init --dir /wt"}}, false},
		{"fellowship init --plan-skip", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "fellowship init --dir /wt --phase Implement --plan-skip"}}, false},
		{"fellowship state show", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "fellowship state show --json"}}, true},
		{"fellowship state init --claim-lead", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "cd /repo && fellowship state init --claim-lead"}}, true},
		{"isolation self-check", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "git -C /wt rev-parse --path-format=absolute --show-toplevel && git rev-parse --path-format=absolute --git-common-dir"}}, false},
		{"git worktree list", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "git worktree list"}}, false},
		{"git status", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "git -C /wt status --porcelain"}}, false},
		{"git commit", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "git -C /wt commit -am fix"}}, true},
		{"git add", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "git -C /wt add ."}}, true},
		{"git worktree add", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "git worktree add -b x /wt2"}}, true},
		{"heredoc write", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "cat > /wt/calc.py <<'EOF'\ndef f(): pass\nEOF"}}, true},
		{"append redirect", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "echo hi >> /wt/a.txt"}}, true},
		{"sed -i", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "sed -i s/a/b/ /wt/calc.py"}}, true},
		{"gh pr create", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "gh pr create"}}, true},
		{"python", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "python3 -c 'open(\"/wt/x\",\"w\")'"}}, true},
		{"read-only builtins", &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "pwd && ls /wt && cat /wt/README.md | head -5"}}, false},
		{"empty command", &HookInput{ToolName: "Bash"}, false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := BootstrapGuard(c.input, "quest-x", ".fellowship")
			if r.Block != c.block {
				t.Errorf("block = %v, want %v (%s)", r.Block, c.block, r.Message)
			}
		})
	}
}
