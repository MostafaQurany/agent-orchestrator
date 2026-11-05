package repoplan_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/repoplan"
)

const sampleValidPlan = `---
id: %s
title: Sample Plan
status: draft
---

# Sample Plan

## Phase 1: Initiation
### TASK-1: First Task
- [ ] Checklist item 1
`

func setupPlansRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	plansDir := filepath.Join(root, "docs", "ao", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatalf("mkdir plans dir: %v", err)
	}
	return root
}

func writePlan(t *testing.T, root, filename, content string) string {
	t.Helper()
	p := filepath.Join(root, "docs", "ao", "plans", filename)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write plan file %s: %v", filename, err)
	}
	return p
}

func TestLoader_ValidateID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"valid-plan-1", true},
		{"plan", true},
		{"a-b-c", true},
		{"123", true},
		{"plan-123", true},
		{"", false},
		{"-plan", false},
		{"plan-", false},
		{"Plan-1", false},
		{"plan_1", false},
		{"../plan", false},
		{"plan/1", false},
		{"plan\\1", false},
		{"plan\x00", false},
		{"plan.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := repoplan.ValidateID(tt.id); got != tt.valid {
				t.Errorf("ValidateID(%q) = %v, want %v", tt.id, got, tt.valid)
			}
		})
	}
}

func TestLoader_Get(t *testing.T) {
	root := setupPlansRepo(t)

	// Valid plan
	writePlan(t, root, "test-plan.md", sampleValidPlanFormatted("test-plan"))

	plan, err := repoplan.Get(root, "test-plan")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if plan == nil || plan.ID != "test-plan" {
		t.Fatalf("unexpected plan: %+v", plan)
	}

	// Invalid ID
	_, err = repoplan.Get(root, "../escape")
	if !errors.Is(err, repoplan.ErrInvalidID) {
		t.Errorf("expected ErrInvalidID for traversal id, got: %v", err)
	}

	_, err = repoplan.Get(root, "INVALID_UPPER")
	if !errors.Is(err, repoplan.ErrInvalidID) {
		t.Errorf("expected ErrInvalidID for uppercase id, got: %v", err)
	}

	// Missing plan
	_, err = repoplan.Get(root, "non-existent")
	if !errors.Is(err, repoplan.ErrNotFound) {
		t.Errorf("expected ErrNotFound for missing plan, got: %v", err)
	}

	// Mismatched filename and frontmatter ID slug
	writePlan(t, root, "mismatch.md", sampleValidPlanFormatted("different-id"))
	_, err = repoplan.Get(root, "mismatch")
	if !errors.Is(err, repoplan.ErrMalformed) {
		t.Errorf("expected ErrMalformed for mismatched ID slug, got: %v", err)
	}

	// Malformed markdown file
	writePlan(t, root, "bad-format.md", "this is not frontmatter")
	_, err = repoplan.Get(root, "bad-format")
	if !errors.Is(err, repoplan.ErrMalformed) {
		t.Errorf("expected ErrMalformed for non-yaml file, got: %v", err)
	}

	// File too large
	largeContent := strings.Repeat("A", repoplan.MaxPlanFileSize+10)
	writePlan(t, root, "too-large.md", largeContent)
	_, err = repoplan.Get(root, "too-large")
	if !errors.Is(err, repoplan.ErrPlanTooLarge) {
		t.Errorf("expected ErrPlanTooLarge, got: %v", err)
	}
}

func TestLoader_List(t *testing.T) {
	// Missing directory returns empty list, nil error
	emptyRoot := t.TempDir()
	plans, err := repoplan.List(emptyRoot)
	if err != nil {
		t.Fatalf("expected nil error for missing directory, got: %v", err)
	}
	if len(plans) != 0 {
		t.Fatalf("expected 0 plans for missing directory, got: %d", len(plans))
	}

	root := setupPlansRepo(t)

	// Empty plans directory
	plans, err = repoplan.List(root)
	if err != nil {
		t.Fatalf("expected nil error for empty dir, got: %v", err)
	}
	if len(plans) != 0 {
		t.Fatalf("expected 0 plans, got: %d", len(plans))
	}

	// Populate plans and non-plans
	writePlan(t, root, "plan-b.md", sampleValidPlanFormatted("plan-b"))
	writePlan(t, root, "plan-a.md", sampleValidPlanFormatted("plan-a"))
	writePlan(t, root, "notes.txt", "ignore me")
	writePlan(t, root, "README", "ignore me")

	subDir := filepath.Join(root, "docs", "ao", "plans", "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}

	plans, err = repoplan.List(root)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got: %d", len(plans))
	}
	if plans[0].ID != "plan-a" || plans[1].ID != "plan-b" {
		t.Errorf("expected sorted plans [plan-a, plan-b], got: [%s, %s]", plans[0].ID, plans[1].ID)
	}

	// Malformed file in list causes error
	writePlan(t, root, "corrupted.md", "bad content")
	_, err = repoplan.List(root)
	if !errors.Is(err, repoplan.ErrMalformed) {
		t.Errorf("expected ErrMalformed when listing corrupted plan, got: %v", err)
	}
}

func TestLoader_SymlinkEscape(t *testing.T) {
	root := setupPlansRepo(t)
	outsideDir := t.TempDir()
	secretFile := filepath.Join(outsideDir, "secret.md")
	if err := os.WriteFile(secretFile, []byte(sampleValidPlanFormatted("secret")), 0o644); err != nil {
		t.Fatalf("write secret file: %v", err)
	}

	// Symlink in plans pointing outside
	symlinkPath := filepath.Join(root, "docs", "ao", "plans", "secret.md")
	err := os.Symlink(secretFile, symlinkPath)
	if err != nil {
		t.Skipf("symlink creation not supported or permitted: %v", err)
	}

	_, err = repoplan.Get(root, "secret")
	if !errors.Is(err, repoplan.ErrEscapesRoot) {
		t.Errorf("expected ErrEscapesRoot for symlink pointing outside docs/ao/plans, got: %v", err)
	}

	// List also fails when a plan symlink escapes
	_, err = repoplan.List(root)
	if !errors.Is(err, repoplan.ErrEscapesRoot) {
		t.Errorf("expected ErrEscapesRoot in List with escaping symlink, got: %v", err)
	}
}

func TestLoader_DirSymlinkEscape(t *testing.T) {
	outsideDir := t.TempDir()
	outsidePlans := filepath.Join(outsideDir, "plans")
	if err := os.MkdirAll(outsidePlans, 0o755); err != nil {
		t.Fatalf("mkdir outside plans: %v", err)
	}

	secretFile := filepath.Join(outsidePlans, "outside-plan.md")
	if err := os.WriteFile(secretFile, []byte(sampleValidPlanFormatted("outside-plan")), 0o644); err != nil {
		t.Fatalf("write outside plan: %v", err)
	}

	root := t.TempDir()
	docsAoDir := filepath.Join(root, "docs", "ao")
	if err := os.MkdirAll(docsAoDir, 0o755); err != nil {
		t.Fatalf("mkdir docs/ao: %v", err)
	}

	// Symlink docs/ao/plans -> outsideDir/plans
	symlinkPath := filepath.Join(docsAoDir, "plans")
	err := os.Symlink(outsidePlans, symlinkPath)
	if err != nil {
		t.Skipf("symlink creation not supported or permitted: %v", err)
	}

	// Get must reject escaped directory symlink
	_, err = repoplan.Get(root, "outside-plan")
	if !errors.Is(err, repoplan.ErrEscapesRoot) {
		t.Errorf("expected ErrEscapesRoot for dir symlink pointing outside projectRoot in Get, got: %v", err)
	}

	// List must reject escaped directory symlink
	_, err = repoplan.List(root)
	if !errors.Is(err, repoplan.ErrEscapesRoot) {
		t.Errorf("expected ErrEscapesRoot for dir symlink pointing outside projectRoot in List, got: %v", err)
	}
}

func sampleValidPlanFormatted(id string) string {
	return `---
id: ` + id + `
title: ` + id + `
status: draft
---

# ` + id + `

## Phase 1: Initiation
### TASK-1: First Task
- [ ] Checklist item 1
`
}
