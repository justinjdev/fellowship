package autopsy

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	if input == nil {
		return "", fmt.Errorf("input is required")
	}
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

	randBytes := make([]byte, 4)
	if _, err := rand.Read(randBytes); err != nil {
		return "", fmt.Errorf("generating autopsy filename suffix: %w", err)
	}
	filename := fmt.Sprintf("%s-%s-%x.json", now.Format("20060102T150405"), sanitize(input.Quest), randBytes)
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
			return nil, fmt.Errorf("reading autopsy %s: %w", entry.Name(), err)
		}

		ts, err := time.Parse(time.RFC3339, a.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("parsing autopsy timestamp for %s: %w", entry.Name(), err)
		}
		if ts.Before(cutoff) {
			if err := os.Remove(path); err != nil {
				return nil, fmt.Errorf("pruning expired autopsy %s: %w", entry.Name(), err)
			}
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
	trigger, whatFailed, err := inferTrigger(worktreeDir, t)
	if err != nil {
		return "", err
	}
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

func inferTrigger(worktreeDir string, t *tome.QuestTome) (string, string, error) {
	// Check for respawns
	if t.Respawns > 0 {
		return "recovery", fmt.Sprintf("Quest required %d respawn(s)", t.Respawns), nil
	}

	// Check for gate rejections in herald
	tidings, err := herald.Read(worktreeDir, 0)
	if err != nil && !os.IsNotExist(err) {
		return "", "", fmt.Errorf("reading herald: %w", err)
	}
	for i := len(tidings) - 1; i >= 0; i-- {
		if tidings[i].Type == herald.GateRejected {
			detail := tidings[i].Detail
			if detail == "" {
				detail = fmt.Sprintf("Gate rejected at %s phase", tidings[i].Phase)
			}
			return "rejection", detail, nil
		}
	}

	// Check for failed/cancelled status in tome
	if t.Status == "failed" || t.Status == "cancelled" {
		return "abandonment", fmt.Sprintf("Quest %s with status: %s", t.QuestName, t.Status), nil
	}

	return "", "", nil
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
	sort.Strings(modules)
	return modules
}

func matchesFilters(a *Autopsy, opts ScanOptions) bool {
	// File match: exact match, directory containment, or same directory
	for _, queryFile := range opts.Files {
		for _, autopsyFile := range a.Files {
			// Exact match
			if queryFile == autopsyFile {
				return true
			}
			// Directory containment (query is a dir prefix of autopsy file or vice versa)
			if strings.HasSuffix(queryFile, "/") && strings.HasPrefix(autopsyFile, queryFile) {
				return true
			}
			if strings.HasSuffix(autopsyFile, "/") && strings.HasPrefix(queryFile, autopsyFile) {
				return true
			}
			// Same directory (skip root-level files)
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
	for _, c := range []string{" ", "/", "\\", ":", "*", "?", "\"", "<", ">", "|"} {
		s = strings.ReplaceAll(s, c, "-")
	}
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

// DefaultExpiryDays is the default autopsy TTL when not configured.
const DefaultExpiryDays = 90
