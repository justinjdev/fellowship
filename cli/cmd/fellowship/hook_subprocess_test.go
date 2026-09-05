package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// packageDir returns the directory this test file lives in, regardless of
// the test binary's current working directory (which other tests in this
// package chdir around) — `go build .` needs to run from here.
func packageDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to resolve this file's path")
	}
	return filepath.Dir(file)
}

// buildFellowshipBinary compiles the fellowship CLI to a temp directory and
// returns its path. Building once and running it as a real subprocess is the
// only way to exercise main()'s storeOpenExit path end to end — the exit
// code it produces before any hook-specific logic runs.
func buildFellowshipBinary(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "fellowship-test-bin")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = packageDir(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return binPath
}

// runFellowshipHook runs the built binary as `fellowship hook <name>` in dir
// and returns its exit code, per cmd.ProcessState.ExitCode() — not by
// inspecting the error Run() returns, since a nonzero exit is the expected,
// successful outcome for these cases.
func runFellowshipHook(t *testing.T, binPath, dir, hookName string) (exitCode int, stderr string) {
	t.Helper()
	cmd := exec.Command(binPath, "hook", hookName)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(nil) // gate-guard tolerates empty/malformed input
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	_ = cmd.Run()
	if cmd.ProcessState == nil {
		t.Fatal("process did not complete — no ProcessState")
	}
	return cmd.ProcessState.ExitCode(), errBuf.String()
}

// End-to-end coverage of storeOpenExit's two extremes for a gate hook,
// through the real built binary rather than runHookWith: no store at all
// must allow (nothing to enforce, and must not create one just by being
// run), while a store that exists but can't be read must fail closed.
func TestHookGateGuard_StoreOpenExitCodes(t *testing.T) {
	binPath := buildFellowshipBinary(t)

	t.Run("no store exits zero", func(t *testing.T) {
		repo := newMainRepo(t)

		code, stderr := runFellowshipHook(t, binPath, repo, "gate-guard")
		if code != 0 {
			t.Errorf("exit code = %d, want 0 (stderr: %s)", code, stderr)
		}
		if _, err := os.Stat(filepath.Join(repo, ".fellowship", "fellowship.db")); err == nil {
			t.Error("hook gate-guard must not create a store just by running")
		}
	})

	t.Run("corrupt store exits two", func(t *testing.T) {
		repo := newMainRepo(t)
		dataDir := filepath.Join(repo, ".fellowship")
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dataDir, "fellowship.db"), []byte("not a sqlite database"), 0o644); err != nil {
			t.Fatal(err)
		}

		code, stderr := runFellowshipHook(t, binPath, repo, "gate-guard")
		if code != 2 {
			t.Errorf("exit code = %d, want 2 (stderr: %s)", code, stderr)
		}
	})
}
