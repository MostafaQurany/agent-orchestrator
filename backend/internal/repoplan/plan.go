package repoplan

import "fmt"

// Status represents plan approval status.
type Status string

const (
	StatusDraft    Status = "draft"
	StatusApproved Status = "approved"
)

// IsValid reports whether status is recognized.
func (s Status) IsValid() bool {
	return s == StatusDraft || s == StatusApproved
}

// ChecklistItem is one task checklist checkbox.
type ChecklistItem struct {
	Line    int    `json:"line"`
	Checked bool   `json:"checked"`
	Text    string `json:"text"`
}

// Task represents one task in phase.
type Task struct {
	Line        int             `json:"line"`
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	Checklist   []ChecklistItem `json:"checklist"`
}

// IsComplete reports whether all checklist items are checked.
func (t *Task) IsComplete() bool {
	if len(t.Checklist) == 0 {
		return false
	}
	for _, item := range t.Checklist {
		if !item.Checked {
			return false
		}
	}
	return true
}

// Phase represents one plan phase.
type Phase struct {
	Line        int    `json:"line"`
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Tasks       []Task `json:"tasks"`
}

// Plan represents parsed markdown plan document.
type Plan struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Status      Status  `json:"status"`
	Description string  `json:"description,omitempty"`
	Phases      []Phase `json:"phases"`
}

// FindTask finds task by ID across all phases.
func (p *Plan) FindTask(id string) *Task {
	for pi := range p.Phases {
		for ti := range p.Phases[pi].Tasks {
			if p.Phases[pi].Tasks[ti].ID == id {
				return &p.Phases[pi].Tasks[ti]
			}
		}
	}
	return nil
}

// TaskCount returns total tasks count.
func (p *Plan) TaskCount() int {
	total := 0
	for _, ph := range p.Phases {
		total += len(ph.Tasks)
	}
	return total
}

// CompletedTaskCount returns count of tasks with all items checked.
func (p *Plan) CompletedTaskCount() int {
	completed := 0
	for _, ph := range p.Phases {
		for i := range ph.Tasks {
			if ph.Tasks[i].IsComplete() {
				completed++
			}
		}
	}
	return completed
}

// ParseError records error with 1-based line number.
type ParseError struct {
	Line    int
	Message string
	Err     error
}

func (e *ParseError) Error() string {
	if e.Line > 0 {
		if e.Err != nil && e.Message != "" {
			return fmt.Sprintf("line %d: %s: %v", e.Line, e.Message, e.Err)
		}
		if e.Message != "" {
			return fmt.Sprintf("line %d: %s", e.Line, e.Message)
		}
		if e.Err != nil {
			return fmt.Sprintf("line %d: %v", e.Line, e.Err)
		}
	}
	if e.Err != nil && e.Message != "" {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "unknown parse error"
}

func (e *ParseError) Unwrap() error {
	return e.Err
}
