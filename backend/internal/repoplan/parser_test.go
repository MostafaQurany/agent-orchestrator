package repoplan

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseExamplePlanFile(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "ao", "plans", "example-plan.md")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	plan, err := ParseBytes(data)
	require.NoError(t, err)
	require.NotNil(t, plan)

	require.Equal(t, "example-plan", plan.ID)
	require.Equal(t, "AO Company MVP Implementation Plan", plan.Title)
	require.Equal(t, StatusApproved, plan.Status)
	require.Len(t, plan.Phases, 3)

	require.Equal(t, 1, plan.Phases[0].Number)
	require.Equal(t, "Domain Models, Parser, and Path Validation", plan.Phases[0].Title)
	require.Len(t, plan.Phases[0].Tasks, 2)
	require.Equal(t, "PARSE-01", plan.Phases[0].Tasks[0].ID)
	require.Equal(t, "PARSE-02", plan.Phases[0].Tasks[1].ID)

	require.Equal(t, 2, plan.Phases[1].Number)
	require.Equal(t, "Execution Gate and Worker Binding", plan.Phases[1].Title)
	require.Len(t, plan.Phases[1].Tasks, 3)
	require.Equal(t, "GATE-01", plan.Phases[1].Tasks[0].ID)
	require.Equal(t, "GATE-02", plan.Phases[1].Tasks[1].ID)
	require.Equal(t, "LAUNCH-01", plan.Phases[1].Tasks[2].ID)

	require.Equal(t, 3, plan.Phases[2].Number)
	require.Equal(t, "Project UI and Desktop Presentation", plan.Phases[2].Title)
	require.Len(t, plan.Phases[2].Tasks, 2)
	require.Equal(t, "UI-01", plan.Phases[2].Tasks[0].ID)
	require.Equal(t, "UI-02", plan.Phases[2].Tasks[1].ID)

	require.Equal(t, 7, plan.TaskCount())
	require.Equal(t, 0, plan.CompletedTaskCount())
}

func TestParseValidPlans(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedID    string
		expectedTitle string
		expectedStat  Status
		phaseCount    int
		taskCount     int
		completed     int
	}{
		{
			name: "minimal draft plan",
			input: `---
id: plan-min
title: Minimal Plan
status: draft
---

## Phase 1: Bootstrap

### TASK-1: Initialize repo
- [ ] Create repository
`,
			expectedID:    "plan-min",
			expectedTitle: "Minimal Plan",
			expectedStat:  StatusDraft,
			phaseCount:    1,
			taskCount:     1,
			completed:     0,
		},
		{
			name: "approved multi-phase plan with H1 and intro H2 in preamble",
			input: `---
id: plan-appr
title: Approved Full Plan
status: approved
---

# Main Doc Title

## Context and Overview
Some introductory notes.

## Phase 1: Foundation

Phase 1 description notes.

### TASK-01: Setup workspace
Setup description.
- [x] Create worktree
- [X] Verify config

### TASK-02: Dependencies
- [x] Run npm ci
- [ ] Run go mod tidy

## Phase 2: Implementation

### TASK-03: Core feature
- [x] Write code
`,
			expectedID:    "plan-appr",
			expectedTitle: "Approved Full Plan",
			expectedStat:  StatusApproved,
			phaseCount:    2,
			taskCount:     3,
			completed:     2,
		},
		{
			name: "CRLF line endings and Unicode content",
			input: "---\r\n" +
				"id: plan-unicode\r\n" +
				"title: 🚀 计划与测试 🎯\r\n" +
				"status: draft\r\n" +
				"---\r\n" +
				"\r\n" +
				"## Phase 1: Initialisierung & Konfiguration\r\n" +
				"\r\n" +
				"### TASK-DE: Prüfschritt ausführen\r\n" +
				"- [ ] Erste Überprüfung durchführen\r\n" +
				"- [x] Zweite Überprüfung abgeschlossen\r\n" +
				"\r\n" +
				"### TASK-JA: 完了タスク\r\n" +
				"- [x] 成功しました\r\n",
			expectedID:    "plan-unicode",
			expectedTitle: "🚀 计划与测试 🎯",
			expectedStat:  StatusDraft,
			phaseCount:    1,
			taskCount:     2,
			completed:     1,
		},
		{
			name: "fenced code blocks with headings and checklists inside",
			input: `---
id: plan-fences
title: Plan with code fences
status: draft
---

## Phase 1: Core

### TASK-1: Scripting
Here is sample code:
` + "```bash" + `
# This bash comment is not a markdown heading
- [ ] echo "not a checklist"
` + "```" + `
- [ ] Run the script
`,
			expectedID:    "plan-fences",
			expectedTitle: "Plan with code fences",
			expectedStat:  StatusDraft,
			phaseCount:    1,
			taskCount:     1,
			completed:     0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := Parse(tc.input)
			require.NoError(t, err)
			require.NotNil(t, plan)
			require.Equal(t, tc.expectedID, plan.ID)
			require.Equal(t, tc.expectedTitle, plan.Title)
			require.Equal(t, tc.expectedStat, plan.Status)
			require.Len(t, plan.Phases, tc.phaseCount)
			require.Equal(t, tc.taskCount, plan.TaskCount())
			require.Equal(t, tc.completed, plan.CompletedTaskCount())
		})
	}
}

func TestParseBytesAndReader(t *testing.T) {
	raw := "\xef\xbb\xbf---\nid: bom-plan\ntitle: BOM Plan\nstatus: draft\n---\n## Phase 1: Init\n### T-1: Run\n- [ ] ok\n"

	plan1, err := ParseBytes([]byte(raw))
	require.NoError(t, err)
	require.Equal(t, "bom-plan", plan1.ID)

	plan2, err := ParseReader(strings.NewReader(raw))
	require.NoError(t, err)
	require.Equal(t, "bom-plan", plan2.ID)

	task := plan2.FindTask("T-1")
	require.NotNil(t, task)
	require.Equal(t, "Run", task.Title)
	require.False(t, task.IsComplete())

	missing := plan2.FindTask("NONEXISTENT")
	require.Nil(t, missing)
}

func TestParseErrorsTable(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		errLine     int
		errContains string
	}{
		{
			name:        "empty document",
			input:       "",
			errLine:     1,
			errContains: "empty plan document",
		},
		{
			name:        "missing frontmatter start delimiter",
			input:       "# Title without frontmatter",
			errLine:     1,
			errContains: "missing frontmatter opening delimiter",
		},
		{
			name: "unclosed frontmatter delimiter",
			input: `---
id: plan-1
title: test
status: draft
`,
			errLine:     5,
			errContains: "unclosed frontmatter delimiter",
		},
		{
			name: "frontmatter missing id",
			input: `---
title: test
status: draft
---
## Phase 1: P
### T-1: T
- [ ] c
`,
			errLine:     1,
			errContains: "missing required metadata 'id'",
		},
		{
			name: "frontmatter empty id",
			input: `---
id: ""
title: test
status: draft
---
## Phase 1: P
### T-1: T
- [ ] c
`,
			errLine:     2,
			errContains: "metadata 'id' cannot be empty",
		},
		{
			name: "frontmatter id too short",
			input: `---
id: ab
title: test
status: draft
---
## Phase 1: P
### T-1: T
- [ ] c
`,
			errLine:     2,
			errContains: "must be a 3-64 character lowercase slug",
		},
		{
			name: "frontmatter id uppercase disallowed",
			input: `---
id: Plan-1
title: test
status: draft
---
## Phase 1: P
### T-1: T
- [ ] c
`,
			errLine:     2,
			errContains: "must be a 3-64 character lowercase slug",
		},
		{
			name: "frontmatter id underscore disallowed",
			input: `---
id: plan_1
title: test
status: draft
---
## Phase 1: P
### T-1: T
- [ ] c
`,
			errLine:     2,
			errContains: "must be a 3-64 character lowercase slug",
		},
		{
			name: "frontmatter id leading hyphen disallowed",
			input: `---
id: -plan-1
title: test
status: draft
---
## Phase 1: P
### T-1: T
- [ ] c
`,
			errLine:     2,
			errContains: "must be a 3-64 character lowercase slug",
		},
		{
			name: "frontmatter duplicate id key",
			input: `---
id: plan-one
id: plan-two
title: test
status: draft
---
## Phase 1: P
### T-1: T
- [ ] c
`,
			errLine:     3,
			errContains: "duplicate metadata key \"id\"",
		},
		{
			name: "frontmatter duplicate title key",
			input: `---
id: plan-one
title: First Title
title: Second Title
status: draft
---
## Phase 1: P
### T-1: T
- [ ] c
`,
			errLine:     4,
			errContains: "duplicate metadata key \"title\"",
		},
		{
			name: "frontmatter duplicate status key",
			input: `---
id: plan-one
title: Title
status: draft
status: approved
---
## Phase 1: P
### T-1: T
- [ ] c
`,
			errLine:     5,
			errContains: "duplicate metadata key \"status\"",
		},
		{
			name: "frontmatter missing title",
			input: `---
id: plan-1
status: draft
---
## Phase 1: P
### T-1: T
- [ ] c
`,
			errLine:     1,
			errContains: "missing required metadata 'title'",
		},
		{
			name: "frontmatter empty title",
			input: `---
id: plan-1
title: "   "
status: draft
---
## Phase 1: P
### T-1: T
- [ ] c
`,
			errLine:     3,
			errContains: "metadata 'title' cannot be empty",
		},
		{
			name: "frontmatter title too long",
			input: `---
id: plan-1
title: "This title is intentionally way too long because the maximum allowed title length in our specification is exactly one hundred and twenty characters long!"
status: draft
---
## Phase 1: P
### T-1: T
- [ ] c
`,
			errLine:     3,
			errContains: "invalid title length",
		},
		{
			name: "frontmatter missing status",
			input: `---
id: plan-1
title: test
---
## Phase 1: P
### T-1: T
- [ ] c
`,
			errLine:     1,
			errContains: "missing required metadata 'status'",
		},
		{
			name: "frontmatter invalid status",
			input: `---
id: plan-1
title: test
status: in_progress
---
## Phase 1: P
### T-1: T
- [ ] c
`,
			errLine:     4,
			errContains: "invalid status \"in_progress\"",
		},
		{
			name: "plan has no phases",
			input: `---
id: plan-1
title: test
status: draft
---

Just some introductory text.
`,
			errLine:     8,
			errContains: "plan has no phases",
		},
		{
			name: "bad nesting: H1 heading after phases begin",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
### T-1: Start
- [ ] ok

# Extra H1 Heading After Phase
`,
			errLine:     11,
			errContains: "unexpected top-level heading",
		},
		{
			name: "bad nesting: non-phase H2 after phases begin",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
### T-1: Start
- [ ] ok

## Random Section
### T-2: Next
- [ ] ok
`,
			errLine:     11,
			errContains: "expected '## Phase N: title'",
		},
		{
			name: "bad nesting: H4 heading",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
### T-1: Start
- [ ] ok
#### Deep Heading
`,
			errLine:     10,
			errContains: "unexpected heading level 4",
		},
		{
			name: "bad nesting: task before phase",
			input: `---
id: plan-1
title: test
status: draft
---

### T-1: Orphan Task
- [ ] ok

## Phase 1: Setup
`,
			errLine:     7,
			errContains: "before any phase (bad nesting)",
		},
		{
			name: "bad nesting: checklist outside task",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
- [ ] checklist directly under phase

### T-1: Valid
- [ ] ok
`,
			errLine:     8,
			errContains: "outside task (bad nesting)",
		},
		{
			name: "malformed heading: missing space after hash",
			input: `---
id: plan-1
title: test
status: draft
---

##Phase 1: Setup
### T-1: Start
- [ ] ok
`,
			errLine:     7,
			errContains: "missing space after '#'",
		},
		{
			name: "malformed phase heading: phase 0 rejected",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 0: Setup
### T-1: Start
- [ ] ok
`,
			errLine:     7,
			errContains: "invalid phase number 0: must be positive (>= 1)",
		},
		{
			name: "malformed phase heading: invalid number",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase one: Setup
### T-1: Start
- [ ] ok
`,
			errLine:     7,
			errContains: "invalid phase number",
		},
		{
			name: "malformed phase heading: missing colon",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1 Setup
### T-1: Start
- [ ] ok
`,
			errLine:     7,
			errContains: "missing colon separator",
		},
		{
			name: "malformed phase heading: empty title",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1:
### T-1: Start
- [ ] ok
`,
			errLine:     7,
			errContains: "empty phase title",
		},
		{
			name: "phase numbers not strictly increasing (equal)",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: First
### T-1: Start
- [ ] ok

## Phase 1: Duplicate
### T-2: Start
- [ ] ok
`,
			errLine:     11,
			errContains: "phase number 1 must be strictly greater than previous phase number 1",
		},
		{
			name: "phase numbers not strictly increasing (decreasing)",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 2: First
### T-1: Start
- [ ] ok

## Phase 1: Decreasing
### T-2: Start
- [ ] ok
`,
			errLine:     11,
			errContains: "phase number 1 must be strictly greater than previous phase number 2",
		},
		{
			name: "phase has no tasks before next phase",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Empty
## Phase 2: Next
### T-2: Start
- [ ] ok
`,
			errLine:     7,
			errContains: "phase 1 has no tasks",
		},
		{
			name: "phase has no tasks at EOF",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Empty
`,
			errLine:     7,
			errContains: "phase 1 has no tasks",
		},
		{
			name: "malformed task heading: missing colon",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
### TASK-1 Missing Colon
- [ ] ok
`,
			errLine:     8,
			errContains: "missing colon separator",
		},
		{
			name: "malformed task heading: empty ID",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
### : Title
- [ ] ok
`,
			errLine:     8,
			errContains: "empty task ID",
		},
		{
			name: "malformed task heading: lowercase ID disallowed",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
### task-1: Title
- [ ] ok
`,
			errLine:     8,
			errContains: "must be uppercase alphanumeric",
		},
		{
			name: "malformed task heading: whitespace in ID",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
### TASK 1: Title
- [ ] ok
`,
			errLine:     8,
			errContains: "must be uppercase alphanumeric",
		},
		{
			name: "malformed task heading: empty title",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
### TASK-1:
- [ ] ok
`,
			errLine:     8,
			errContains: "empty task title",
		},
		{
			name: "duplicate task ID",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
### TASK-1: First
- [ ] ok

### TASK-1: Duplicate
- [ ] ok
`,
			errLine:     11,
			errContains: "duplicate task ID \"TASK-1\"",
		},
		{
			name: "missing checklist: task before next task",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
### TASK-1: No checklist
### TASK-2: Has checklist
- [ ] ok
`,
			errLine:     8,
			errContains: "task \"TASK-1\" has no checklist items",
		},
		{
			name: "missing checklist: task before next phase",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
### TASK-1: No checklist
## Phase 2: Other
### TASK-2: Has checklist
- [ ] ok
`,
			errLine:     8,
			errContains: "task \"TASK-1\" has no checklist items",
		},
		{
			name: "missing checklist: task at EOF",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
### TASK-1: No checklist
`,
			errLine:     8,
			errContains: "task \"TASK-1\" has no checklist items",
		},
		{
			name: "malformed checklist item: empty brackets",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
### TASK-1: Check
- [] invalid syntax
`,
			errLine:     9,
			errContains: "malformed checklist item",
		},
		{
			name: "malformed checklist item: empty text",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
### TASK-1: Check
- [ ]
`,
			errLine:     9,
			errContains: "checklist item text cannot be empty",
		},
		{
			name: "unclosed code block at EOF",
			input: `---
id: plan-1
title: test
status: draft
---

## Phase 1: Setup
### TASK-1: Check
- [ ] ok
` + "```go\nfunc unclosed() {",
			errLine:     11,
			errContains: "unclosed code block",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := Parse(tc.input)
			require.Error(t, err)
			require.Nil(t, plan)

			var parseErr *ParseError
			require.True(t, errors.As(err, &parseErr), "expected *ParseError, got %T: %v", err, err)
			require.Equal(t, tc.errLine, parseErr.Line, "expected error on line %d, got line %d (%v)", tc.errLine, parseErr.Line, err)
			require.Contains(t, parseErr.Error(), tc.errContains)
		})
	}
}
