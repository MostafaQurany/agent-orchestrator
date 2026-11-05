---
id: example-plan
title: AO Company MVP Implementation Plan
status: approved
---

# AO Company MVP Implementation Plan

This operational plan defines the engineering tasks required to implement the Agent Orchestrator (AO) Company MVP. It demonstrates the canonical Git-versioned Markdown plan schema used by AO to coordinate structured, auditable agent workflows.

## Context and Goals

AO is expanding from ad-hoc session supervision into a structured autonomous company execution engine. This plan establishes the foundation:
- Parse and validate Git-versioned Markdown plans in `docs/ao/plans/*.md`.
- Provide read-only plan inspection endpoints over the loopback daemon API.
- Enforce the opt-in `requirePlan` project gate before spawning worker sessions.
- Bind worker sessions directly to specific plan tasks with injected checklists.
- Provide a clean, dark-first desktop UI in the Electron supervisor matching `DESIGN.md`.

---

## Phase 1: Domain Models, Parser, and Path Validation

Phase 1 establishes the parser, validation rules, and path safety invariants for reading and interpreting plan files directly from the repository tree.

### PARSE-01: Plan Frontmatter and Markdown AST Parser

Implement a parser in Go that extracts structured metadata, phase groupings, task definitions, and checklists from Git-versioned Markdown files.

- [ ] Define `Plan`, `Phase`, `Task`, and `ChecklistItem` types in `backend/internal/domain/plan.go`.
- [ ] Implement YAML frontmatter parser supporting `id`, `title`, and `status` (`draft` | `approved`).
- [ ] Implement Markdown heading scanner for `## Phase N: <Title>` and `### <TASK-ID>: <Title>`.
- [ ] Extract checklist items (`- [ ] ` and `- [x] `) and bind them to their parent task.
- [ ] Enforce validation rules: fail if `id` does not match filename slug, if `status` is not `draft` or `approved`, if task IDs are duplicated, or if any task lacks a checklist item.
- [ ] Add unit test suite in `backend/internal/domain/plan_test.go` covering valid plans and malformed edge cases.

### PARSE-02: Path Confinement and Security Bounds

Ensure all plan filesystem access is strictly bounded to the project repository's `docs/ao/plans/` directory, preventing directory traversal and symlink escapes.

- [ ] Implement path validation in `backend/internal/domain/plan_path.go`.
- [ ] Sanitize plan IDs against allowed character set `^[a-z0-9][a-z0-9-]*[a-z0-9]$`.
- [ ] Explicitly reject `..`, absolute paths, backslashes, and null bytes.
- [ ] Resolve evaluated symlinks with `filepath.EvalSymlinks` and assert they remain within `<repoRoot>/docs/ao/plans/`.
- [ ] Add table-driven path traversal security tests in `backend/internal/domain/plan_path_test.go`.

---

## Phase 2: Execution Gate and Worker Binding

Phase 2 introduces the opt-in `requirePlan` project configuration flag, gate validation in the session manager, and task context injection into worker sessions.

### GATE-01: Project Configuration requirePlan Flag

Add the `requirePlan` boolean flag to `ProjectConfig` and expose it through the storage layer and project settings API.

- [ ] Add `RequirePlan bool json:"requirePlan,omitempty"` to `domain.ProjectConfig` in `backend/internal/domain/projectconfig.go`.
- [ ] Ensure default value is `false` in `DefaultProjectConfig()` to preserve existing project behavior.
- [ ] Update `UpdateProjectConfigRequest` in `backend/internal/httpd/controllers/dto.go` to accept `requirePlan`.
- [ ] Propose SQLite migration adding `plan_id` and `task_id` columns to `sessions` table (`backend/internal/storage/sqlite/migrations/`).
- [ ] Write unit tests verifying round-trip persistence of `requirePlan` in project config JSON.

### GATE-02: Spawn Enforcement Gate in Session Manager

Integrate plan gate verification into the daemon's session spawn pipeline, blocking unauthorized worker creation when `requirePlan` is active.

- [ ] Add `PlanID` and `TaskID` string fields to `ports.SpawnConfig` (`backend/internal/ports/session.go`) and `controllers.SpawnSessionRequest`.
- [ ] Update `SessionManager.Spawn()` to evaluate project `RequirePlan`.
- [ ] If `RequirePlan` is `true`: verify `PlanID` and `TaskID` are non-empty, load plan from `docs/ao/plans/`, assert plan status is `approved`, and verify `TaskID` exists in the plan.
- [ ] Return classified `apierr.Invalid("PLAN_GATE_BLOCKED", ...)` or `apierr.NotFound("TASK_NOT_FOUND", ...)` upon failure.
- [ ] Ensure standalone workers (no `ProjectID`) bypass project plan gates cleanly.
- [ ] Add integration tests in `backend/internal/session_manager/spawn_plan_gate_test.go`.

### LAUNCH-01: Plan-Bound Worker Context Injection

Inject structured task requirements, checklists, and plan boundaries into worker agent environments at spawn time.

- [ ] Build task briefing helper in `backend/internal/service/session/plan_brief.go`.
- [ ] Construct initial prompt preamble containing Plan Title, Phase, Task ID, Task Title, and pending Checklist items.
- [ ] Append standing instructions: agent must focus strictly on task items, record progress against the checklist, and avoid unapproved architectural changes.
- [ ] Persist `plan_id` and `task_id` into session record via proposed migration columns.
- [ ] Expose bound plan and task identity in `SessionResponse` DTO for UI consumption.

---

## Phase 3: Project UI and Desktop Presentation

Phase 3 implements the desktop user interface for browsing plans, tracking task status, launching plan-bound workers, and configuring project plan requirements.

### UI-01: Project Plans Route and List Surface

Add a first-class read-only Plans surface to the project routes in Electron following `DESIGN.md` conventions.

- [ ] Add project plans route `frontend/src/renderer/routes/_shell.projects.$projectId_.plans.tsx`.
- [ ] Create `PlansList` component using shared list container with 1px hairline dividers (`--color-border`).
- [ ] Display plan row: status glyph dot (green for `approved`, amber for `draft`), plan ID in monospace, title, phase/task summary counts, and tabular updated timestamp.
- [ ] Implement empty state ("No plans found in docs/ao/plans/"), calm loading state, and in-place error banner for malformed plans.
- [ ] Ensure full keyboard navigation (`:focus-visible` ring) and dark/light token adherence.

### UI-02: Plan Detail Viewer and Task Launch Affordance

Create the plan inspection view showing phases, tasks, checklists, and single-click worker launch actions.

- [ ] Implement `PlanDetailView` rendering Markdown context, phases, and task items.
- [ ] Render task checklists displaying completed (`[x]`) and pending (`[ ]`) items.
- [ ] Add "Start Task" action on each task row using `Button` primitive.
- [ ] Clicking "Start Task" navigates to New Task composer with project, plan ID, task ID, and prompt pre-populated.
- [ ] Add `requirePlan` toggle switch in `ProjectSettingsEditor.tsx` with concise, sentence-case explanation.
