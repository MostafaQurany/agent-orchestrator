package repoplan

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

var (
	planIDRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
	taskIDRegex = regexp.MustCompile(`^[A-Z0-9_-]+$`)
)

// Parse parses markdown plan from string.
func Parse(input string) (*Plan, error) {
	if strings.TrimSpace(input) == "" {
		return nil, &ParseError{Line: 1, Message: "empty plan document"}
	}

	// Normalize BOM and line endings.
	cleaned := strings.TrimPrefix(input, "\ufeff")
	cleaned = strings.ReplaceAll(cleaned, "\r\n", "\n")
	cleaned = strings.ReplaceAll(cleaned, "\r", "\n")
	lines := strings.Split(cleaned, "\n")

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, &ParseError{Line: 1, Message: "missing frontmatter opening delimiter '---'"}
	}

	closingIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closingIdx = i
			break
		}
	}
	if closingIdx == -1 {
		return nil, &ParseError{Line: len(lines), Message: "unclosed frontmatter delimiter '---'"}
	}

	// Parse YAML frontmatter.
	fmLines := strings.Join(lines[0:closingIdx+1], "\n")
	var rootNode yaml.Node
	if err := yaml.Unmarshal([]byte(fmLines), &rootNode); err != nil {
		line := extractYAMLLine(err)
		return nil, &ParseError{Line: line, Message: "invalid YAML frontmatter", Err: err}
	}

	if len(rootNode.Content) == 0 {
		return nil, &ParseError{Line: 1, Message: "missing required metadata (empty frontmatter)"}
	}

	mapping := rootNode.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil, &ParseError{Line: 1, Message: "frontmatter must be a YAML mapping"}
	}

	var (
		id, title, status                        string
		idKeyLine, idValLine                     int
		titleKeyLine, titleValLine               int
		statusKeyLine, statusValLine             int
		hasID, hasTitle, hasStatus               bool
		seenFMKeys                               = make(map[string]int)
	)

	for i := 0; i < len(mapping.Content); i += 2 {
		keyNode := mapping.Content[i]
		valNode := mapping.Content[i+1]

		switch keyNode.Value {
		case "id":
			if prevLine, exists := seenFMKeys["id"]; exists {
				return nil, &ParseError{Line: keyNode.Line, Message: fmt.Sprintf("duplicate metadata key \"id\" (first declared on line %d)", prevLine)}
			}
			seenFMKeys["id"] = keyNode.Line
			hasID = true
			idKeyLine = keyNode.Line
			idValLine = valNode.Line
			if valNode.Kind != yaml.ScalarNode {
				return nil, &ParseError{Line: valNode.Line, Message: "metadata 'id' must be scalar string"}
			}
			id = strings.TrimSpace(valNode.Value)
		case "title":
			if prevLine, exists := seenFMKeys["title"]; exists {
				return nil, &ParseError{Line: keyNode.Line, Message: fmt.Sprintf("duplicate metadata key \"title\" (first declared on line %d)", prevLine)}
			}
			seenFMKeys["title"] = keyNode.Line
			hasTitle = true
			titleKeyLine = keyNode.Line
			titleValLine = valNode.Line
			if valNode.Kind != yaml.ScalarNode {
				return nil, &ParseError{Line: valNode.Line, Message: "metadata 'title' must be scalar string"}
			}
			title = strings.TrimSpace(valNode.Value)
		case "status":
			if prevLine, exists := seenFMKeys["status"]; exists {
				return nil, &ParseError{Line: keyNode.Line, Message: fmt.Sprintf("duplicate metadata key \"status\" (first declared on line %d)", prevLine)}
			}
			seenFMKeys["status"] = keyNode.Line
			hasStatus = true
			statusKeyLine = keyNode.Line
			statusValLine = valNode.Line
			if valNode.Kind != yaml.ScalarNode {
				return nil, &ParseError{Line: valNode.Line, Message: "metadata 'status' must be scalar string"}
			}
			status = strings.TrimSpace(valNode.Value)
		}
	}

	if !hasID {
		return nil, &ParseError{Line: 1, Message: "missing required metadata 'id'"}
	}
	if id == "" {
		return nil, &ParseError{Line: idKeyLine, Message: "metadata 'id' cannot be empty"}
	}
	if len(id) < 3 || len(id) > 64 || !planIDRegex.MatchString(id) {
		return nil, &ParseError{
			Line:    idValLine,
			Message: fmt.Sprintf("invalid plan id %q: must be a 3-64 character lowercase slug matching ^[a-z0-9][a-z0-9-]*[a-z0-9]$", id),
		}
	}

	if !hasTitle {
		return nil, &ParseError{Line: 1, Message: "missing required metadata 'title'"}
	}
	if title == "" {
		return nil, &ParseError{Line: titleKeyLine, Message: "metadata 'title' cannot be empty"}
	}
	titleLen := utf8.RuneCountInString(title)
	if titleLen < 1 || titleLen > 120 {
		return nil, &ParseError{
			Line:    titleValLine,
			Message: fmt.Sprintf("invalid title length %d: must be 1-120 characters", titleLen),
		}
	}

	if !hasStatus {
		return nil, &ParseError{Line: 1, Message: "missing required metadata 'status'"}
	}
	if status == "" {
		return nil, &ParseError{Line: statusKeyLine, Message: "metadata 'status' cannot be empty"}
	}
	if !Status(status).IsValid() {
		return nil, &ParseError{
			Line:    statusValLine,
			Message: fmt.Sprintf("invalid status %q, must be 'draft' or 'approved'", status),
		}
	}

	plan := &Plan{
		ID:     id,
		Title:  title,
		Status: Status(status),
	}

	var (
		currentPhase   *Phase
		currentTask    *Task
		planDescLines  []string
		phaseDescLines []string
		taskDescLines  []string
		seenTasks      = make(map[string]int)
		inCodeBlock    bool
	)

	appendDesc := func(raw string) {
		if currentTask != nil {
			taskDescLines = append(taskDescLines, raw)
		} else if currentPhase != nil {
			phaseDescLines = append(phaseDescLines, raw)
		} else {
			planDescLines = append(planDescLines, raw)
		}
	}

	for lineIdx := closingIdx + 1; lineIdx < len(lines); lineIdx++ {
		lineNum := lineIdx + 1
		rawLine := lines[lineIdx]
		trimmed := strings.TrimSpace(rawLine)

		// Check code block fence
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock = !inCodeBlock
			appendDesc(rawLine)
			continue
		}

		if inCodeBlock {
			appendDesc(rawLine)
			continue
		}

		if trimmed == "" {
			appendDesc("")
			continue
		}

		// Check for markdown headings
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}

			if level < len(trimmed) && trimmed[level] != ' ' && trimmed[level] != '\t' {
				return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("malformed heading %q: missing space after '#'", trimmed)}
			}

			switch level {
			case 1:
				if currentPhase == nil {
					// H1 allowed in introduction before first phase
					appendDesc(rawLine)
					continue
				}
				return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("unexpected top-level heading %q (bad nesting)", trimmed)}

			case 2:
				headingText := strings.TrimSpace(trimmed[2:])
				// If before first phase and not a Phase heading, allowed in intro
				if currentPhase == nil && !isPhaseHeading(headingText) {
					appendDesc(rawLine)
					continue
				}

				// Once phases begin (or heading is a Phase heading), must match ## Phase N: title
				if !strings.HasPrefix(headingText, "Phase") {
					return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("malformed phase heading %q: expected '## Phase N: title'", trimmed)}
				}
				afterPhase := strings.TrimPrefix(headingText, "Phase")
				if !strings.HasPrefix(afterPhase, " ") && !strings.HasPrefix(afterPhase, "\t") {
					return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("malformed phase heading %q: expected space after 'Phase'", trimmed)}
				}
				afterPhase = strings.TrimSpace(afterPhase)
				nStr, titleStr, hasColon := strings.Cut(afterPhase, ":")
				if !hasColon {
					return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("malformed phase heading %q: missing colon separator", trimmed)}
				}
				nStr = strings.TrimSpace(nStr)
				if nStr == "" {
					return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("malformed phase heading %q: missing phase number", trimmed)}
				}
				phaseNum, err := strconv.Atoi(nStr)
				if err != nil {
					return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("malformed phase heading %q: invalid phase number %q", trimmed, nStr)}
				}
				if phaseNum <= 0 {
					return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("invalid phase number %d: must be positive (>= 1)", phaseNum)}
				}
				if currentPhase != nil && phaseNum <= currentPhase.Number {
					return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("phase number %d must be strictly greater than previous phase number %d", phaseNum, currentPhase.Number)}
				}
				phaseTitle := strings.TrimSpace(titleStr)
				if phaseTitle == "" {
					return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("malformed phase heading %q: empty phase title", trimmed)}
				}

				// Finalize previous task
				if currentTask != nil {
					if len(currentTask.Checklist) == 0 {
						return nil, &ParseError{Line: currentTask.Line, Message: fmt.Sprintf("task %q has no checklist items (missing checklist)", currentTask.ID)}
					}
					currentTask.Description = cleanDesc(taskDescLines)
					taskDescLines = nil
					currentPhase.Tasks = append(currentPhase.Tasks, *currentTask)
					currentTask = nil
				}

				// Finalize previous phase
				if currentPhase != nil {
					if len(currentPhase.Tasks) == 0 {
						return nil, &ParseError{Line: currentPhase.Line, Message: fmt.Sprintf("phase %d has no tasks", currentPhase.Number)}
					}
					currentPhase.Description = cleanDesc(phaseDescLines)
					phaseDescLines = nil
					plan.Phases = append(plan.Phases, *currentPhase)
					currentPhase = nil
				}

				if len(plan.Phases) == 0 && plan.Description == "" && len(planDescLines) > 0 {
					plan.Description = cleanDesc(planDescLines)
					planDescLines = nil
				}

				currentPhase = &Phase{
					Line:   lineNum,
					Number: phaseNum,
					Title:  phaseTitle,
				}
				continue

			case 3:
				// ### TASK-ID: title
				if currentPhase == nil {
					return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("task heading %q before any phase (bad nesting)", trimmed)}
				}

				headingText := strings.TrimSpace(trimmed[3:])
				idStr, titleStr, hasColon := strings.Cut(headingText, ":")
				if !hasColon {
					return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("malformed task heading %q: missing colon separator", trimmed)}
				}
				taskID := strings.TrimSpace(idStr)
				if taskID == "" {
					return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("malformed task heading %q: empty task ID", trimmed)}
				}
				if !taskIDRegex.MatchString(taskID) {
					return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("malformed task heading %q: task ID %q must be uppercase alphanumeric, hyphen, or underscore", trimmed, taskID)}
				}
				taskTitle := strings.TrimSpace(titleStr)
				if taskTitle == "" {
					return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("malformed task heading %q: empty task title", trimmed)}
				}
				if prevLine, exists := seenTasks[taskID]; exists {
					return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("duplicate task ID %q (first declared on line %d)", taskID, prevLine)}
				}
				seenTasks[taskID] = lineNum

				// Finalize previous task
				if currentTask != nil {
					if len(currentTask.Checklist) == 0 {
						return nil, &ParseError{Line: currentTask.Line, Message: fmt.Sprintf("task %q has no checklist items (missing checklist)", currentTask.ID)}
					}
					currentTask.Description = cleanDesc(taskDescLines)
					taskDescLines = nil
					currentPhase.Tasks = append(currentPhase.Tasks, *currentTask)
					currentTask = nil
				} else {
					if len(phaseDescLines) > 0 {
						currentPhase.Description = cleanDesc(phaseDescLines)
						phaseDescLines = nil
					}
				}

				currentTask = &Task{
					Line:  lineNum,
					ID:    taskID,
					Title: taskTitle,
				}
				continue

			default:
				return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("unexpected heading level %d %q (bad nesting)", level, trimmed)}
			}
		}

		// Check checklist line
		isChecklist, checked, checkText, isMalformed := parseChecklistLine(trimmed)
		if isMalformed {
			return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("malformed checklist item %q: must use '- [ ]' or '- [x]'", trimmed)}
		}
		if isChecklist {
			if currentTask == nil {
				return nil, &ParseError{Line: lineNum, Message: fmt.Sprintf("checklist item %q outside task (bad nesting)", trimmed)}
			}
			if checkText == "" {
				return nil, &ParseError{Line: lineNum, Message: "checklist item text cannot be empty"}
			}
			currentTask.Checklist = append(currentTask.Checklist, ChecklistItem{
				Line:    lineNum,
				Checked: checked,
				Text:    checkText,
			})
			continue
		}

		appendDesc(rawLine)
	}

	if inCodeBlock {
		return nil, &ParseError{Line: len(lines), Message: "unclosed code block at end of document"}
	}

	// Finalize trailing task
	if currentTask != nil {
		if len(currentTask.Checklist) == 0 {
			return nil, &ParseError{Line: currentTask.Line, Message: fmt.Sprintf("task %q has no checklist items (missing checklist)", currentTask.ID)}
		}
		currentTask.Description = cleanDesc(taskDescLines)
		currentPhase.Tasks = append(currentPhase.Tasks, *currentTask)
		currentTask = nil
	}

	// Finalize trailing phase
	if currentPhase != nil {
		if len(currentPhase.Tasks) == 0 {
			return nil, &ParseError{Line: currentPhase.Line, Message: fmt.Sprintf("phase %d has no tasks", currentPhase.Number)}
		}
		currentPhase.Description = cleanDesc(phaseDescLines)
		plan.Phases = append(plan.Phases, *currentPhase)
		currentPhase = nil
	}

	if len(plan.Phases) == 0 {
		return nil, &ParseError{Line: len(lines), Message: "plan has no phases"}
	}

	if plan.Description == "" && len(planDescLines) > 0 {
		plan.Description = cleanDesc(planDescLines)
	}

	return plan, nil
}

// ParseBytes parses markdown plan from byte slice.
func ParseBytes(data []byte) (*Plan, error) {
	if bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) {
		data = data[3:]
	}
	return Parse(string(data))
}

// ParseReader parses markdown plan from an io.Reader.
func ParseReader(r io.Reader) (*Plan, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return ParseBytes(data)
}

func isPhaseHeading(headingText string) bool {
	return headingText == "Phase" ||
		strings.HasPrefix(headingText, "Phase ") ||
		strings.HasPrefix(headingText, "Phase\t") ||
		strings.HasPrefix(headingText, "Phase:")
}

func parseChecklistLine(trimmed string) (isChecklist bool, checked bool, text string, malformed bool) {
	if strings.HasPrefix(trimmed, "- [ ] ") {
		return true, false, strings.TrimSpace(trimmed[6:]), false
	}
	if strings.HasPrefix(trimmed, "- [x] ") || strings.HasPrefix(trimmed, "- [X] ") {
		return true, true, strings.TrimSpace(trimmed[6:]), false
	}
	if trimmed == "- [ ]" {
		return true, false, "", false
	}
	if trimmed == "- [x]" || trimmed == "- [X]" {
		return true, true, "", false
	}
	if strings.HasPrefix(trimmed, "- [") || strings.HasPrefix(trimmed, "* [") {
		return false, false, "", true
	}
	return false, false, "", false
}

func cleanDesc(lines []string) string {
	joined := strings.Join(lines, "\n")
	return strings.TrimSpace(joined)
}

func extractYAMLLine(err error) int {
	if err == nil {
		return 1
	}
	msg := err.Error()
	idx := strings.Index(msg, "line ")
	if idx >= 0 {
		sub := msg[idx+len("line "):]
		colon := strings.Index(sub, ":")
		if colon > 0 {
			if n, parseErr := strconv.Atoi(sub[:colon]); parseErr == nil {
				return n
			}
		}
	}
	return 1
}
