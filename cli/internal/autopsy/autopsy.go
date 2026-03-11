package autopsy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/herald"
	"github.com/justinjdev/fellowship/cli/internal/tome"
)

const autopsyDir = "autopsies"

// Autopsy represents a structured failure record.
type Autopsy struct {
	Version    int      `json:"version"`
	Timestamp  string   `json:"ts"`
	Quest      string   `json:"quest"`
	Task       string   `json:"task"`
	Phase      string   `json:"phase"`
	Trigger    string   `json:"trigger"` // "recovery", "rejection", "abandonment"
	Files      []string `json:"files"`
	Modules    []string `json:"modules"`
	WhatFailed string   `json:"what_failed"`
	Resolution string   `json:"resolution,omitempty"`
	Tags       []string `json:"tags"`
}

// CreateInput is the subset of fields the caller provides; version and timestamp are filled in.
type CreateInput struct {
	Quest      string   `json:"quest"`
	Task       string   `json:"task"`
	Phase      string   `json:"phase"`
	Trigger    string   `json:"trigger"`
	Files      []string `json:"files"`
	Modules    []string `json:"modules"`
	WhatFailed string   `json:"what_failed"`
	Resolution string   `json:"resolution,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

var validTriggers = map[string]bool{
	"recovery":    true,
	"rejection":   true,
	"abandonment": true,
}

// Create validates input, fills in version/timestamp, and writes to the autopsies directory.
func Create(repoRoot string, input *CreateInput) (string, error) {
	if input.Quest == "" {
		return "", fmt.Errorf("quest is required")
	}
	if input.WhatFailed == "" {
		return "", fmt.Errorf("what_failed is required")
	}
	if !validTriggers[input.Trigger] {
		return "", fmt.Errorf("invalid trigger %q (must be recovery, rejection, or abandonment)", input.Trigger)
	}

	now := time.Now().UTC()
	a := &Autopsy{
		Version:    1,
		Timestamp:  now.Format(time.RFC3339),
		Quest:      input.Quest,
		Task:       input.Task,
		Phase:      input.Phase,
		Trigger:    input.Trigger,
		Files:      input.Files,
		Modules:    input.Modules,
		WhatFailed: input.WhatFailed,
		Resolution: input.Resolution,
		Tags:       input.Tags,
	}
	if a.Files == nil {
		a.Files = []string{}
	}
	if a.Modules == nil {
		a.Modules = []string{}
	}
	if a.Tags == nil {
		a.Tags = []string{}
	}

	dir := filepath.Join(repoRoot, datadir.Name(), autopsyDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating autopsies directory: %w", err)
	}

	filename := fmt.Sprintf("%s-%s.json", now.Format("20060102T150405"), sanitize(input.Quest))
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling autopsy: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("writing autopsy: %w", err)
	}
	return path, nil
}

// ScanOptions configures which autopsies to match.
type ScanOptions struct {
	Files   []string
	Modules []string
	Tags    []string
}

// Scan reads all autopsies from the repo root, prunes expired ones, and returns matches.
func Scan(repoRoot string, opts ScanOptions, expiryDays int) ([]Autopsy, error) {
	if len(opts.Files) == 0 && len(opts.Modules) == 0 && len(opts.Tags) == 0 {
		return nil, fmt.Errorf("at least one of --files, --modules, or --tags is required")
	}

	dir := filepath.Join(repoRoot, datadir.Name(), autopsyDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Autopsy{}, nil
		}
		return nil, fmt.Errorf("reading autopsies directory: %w", err)
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -expiryDays)
	var matches []Autopsy

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		a, err := loadAutopsy(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: skipping corrupt autopsy %s: %v\n", entry.Name(), err)
			continue
		}

		// Prune expired or unparseable timestamps
		ts, err := time.Parse(time.RFC3339, a.Timestamp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: skipping autopsy %s with bad timestamp: %v\n", entry.Name(), err)
			continue
		}
		if ts.Before(cutoff) {
			os.Remove(path)
			continue
		}

		if matchesFilters(a, opts) {
			matches = append(matches, *a)
		}
	}

	if matches == nil {
		matches = []Autopsy{}
	}
	return matches, nil
}

// Infer reconstructs a best-effort autopsy from a quest worktree's external signals.
func Infer(worktreeDir, repoRoot string) (string, error) {
	tomePath := filepath.Join(worktreeDir, datadir.Name(), "quest-tome.json")
	t, err := tome.Load(tomePath)
	if err != nil {
		return "", fmt.Errorf("loading tome: %w", err)
	}

	// Determine trigger from signals
	trigger, whatFailed := inferTrigger(worktreeDir, t)
	if trigger == "" {
		return "", fmt.Errorf("no failure signals found in worktree")
	}

	// Derive modules from files_touched
	modules := inferModules(t.FilesTouched)

	input := &CreateInput{
		Quest:      t.QuestName,
		Task:       t.Task,
		Phase:      inferPhase(t),
		Trigger:    trigger,
		Files:      t.FilesTouched,
		Modules:    modules,
		WhatFailed: whatFailed,
	}

	return Create(repoRoot, input)
}

func inferTrigger(worktreeDir string, t *tome.QuestTome) (string, string) {
	// Check for respawns
	if t.Respawns > 0 {
		return "recovery", fmt.Sprintf("Quest required %d respawn(s)", t.Respawns)
	}

	// Check for gate rejections in herald
	tidings, _ := herald.Read(worktreeDir, 0)
	for i := len(tidings) - 1; i >= 0; i-- {
		if tidings[i].Type == herald.GateRejected {
			detail := tidings[i].Detail
			if detail == "" {
				detail = fmt.Sprintf("Gate rejected at %s phase", tidings[i].Phase)
			}
			return "rejection", detail
		}
	}

	// Check for failed/cancelled status in tome
	if t.Status == "failed" || t.Status == "cancelled" {
		return "abandonment", fmt.Sprintf("Quest %s with status: %s", t.QuestName, t.Status)
	}

	return "", ""
}

func inferPhase(t *tome.QuestTome) string {
	if len(t.PhasesCompleted) > 0 {
		return t.PhasesCompleted[len(t.PhasesCompleted)-1].Phase
	}
	return "unknown"
}

func inferModules(files []string) []string {
	seen := map[string]bool{}
	for _, f := range files {
		parts := strings.Split(filepath.ToSlash(f), "/")
		if len(parts) >= 2 {
			// Use the first directory component as the module
			mod := parts[0]
			if !seen[mod] {
				seen[mod] = true
			}
		}
	}
	modules := make([]string, 0, len(seen))
	for m := range seen {
		modules = append(modules, m)
	}
	return modules
}

func matchesFilters(a *Autopsy, opts ScanOptions) bool {
	// File path prefix match
	for _, queryFile := range opts.Files {
		for _, autopsyFile := range a.Files {
			if strings.HasPrefix(autopsyFile, queryFile) || strings.HasPrefix(queryFile, autopsyFile) {
				return true
			}
			// Also match if they share a directory prefix (skip root-level files)
			queryDir := filepath.Dir(queryFile)
			aDir := filepath.Dir(autopsyFile)
			if queryDir != "." && aDir != "." && queryDir == aDir {
				return true
			}
		}
	}

	// Module match
	for _, queryMod := range opts.Modules {
		for _, autopsyMod := range a.Modules {
			if queryMod == autopsyMod {
				return true
			}
		}
	}

	// Tag match
	for _, queryTag := range opts.Tags {
		for _, autopsyTag := range a.Tags {
			if queryTag == autopsyTag {
				return true
			}
		}
	}

	return false
}

func loadAutopsy(path string) (*Autopsy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var a Autopsy
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

// DefaultExpiryDays is the default autopsy TTL when not configured.
const DefaultExpiryDays = 90
