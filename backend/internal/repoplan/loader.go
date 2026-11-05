package repoplan

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Sentinel errors distinguishing failure modes.
var (
	ErrInvalidID   = errors.New("invalid plan id")
	ErrNotFound    = errors.New("plan not found")
	ErrMalformed   = errors.New("malformed plan file")
	ErrPlanTooLarge = errors.New("plan file exceeds maximum size limit")
	ErrEscapesRoot = errors.New("plan path escapes plans directory")
)

// MaxPlanFileSize limits plan file size (1 MB) to prevent denial of service.
const MaxPlanFileSize = 1024 * 1024

// ValidateID checks if the given planID matches the required slug format:
// lowercase alphanumeric and single dashes, starting and ending with alphanumeric.
func ValidateID(planID string) bool {
	if strings.Contains(planID, "/") || strings.Contains(planID, "\\") || strings.Contains(planID, "..") || strings.Contains(planID, "\x00") {
		return false
	}
	return planIDRegex.MatchString(planID)
}

// plansDir returns the relative path from project root to the plans directory.
func plansDirPath(projectRoot string) string {
	return filepath.Join(projectRoot, "docs", "ao", "plans")
}

// resolvePlansDir resolves projectRoot and ensures the plans directory does not escape projectRoot.
func resolvePlansDir(projectRoot string) (string, error) {
	cleanRoot := filepath.Clean(projectRoot)
	realRoot, err := filepath.EvalSymlinks(cleanRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("%w: %q", ErrNotFound, cleanRoot)
		}
		return "", fmt.Errorf("eval root symlinks: %w", err)
	}

	cleanDir := filepath.Clean(plansDirPath(cleanRoot))
	realDir, err := filepath.EvalSymlinks(cleanDir)
	if err != nil {
		return "", err
	}

	relToRoot, err := filepath.Rel(realRoot, realDir)
	if err != nil || strings.HasPrefix(relToRoot, "..") || filepath.IsAbs(relToRoot) {
		return "", fmt.Errorf("%w: plans directory %q escapes project root %q", ErrEscapesRoot, realDir, realRoot)
	}

	return cleanDir, nil
}

// List scans <projectRoot>/docs/ao/plans/*.md and returns all valid parsed plans
// sorted alphabetically by ID.
// If the plans directory does not exist, an empty list and nil error are returned.
// Non-markdown files or subdirectories are ignored.
// If any *.md file is malformed, invalid, or violates safety bounds, an error is returned.
func List(projectRoot string) ([]*Plan, error) {
	dir, err := resolvePlansDir(projectRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []*Plan{}, nil
		}
		if errors.Is(err, ErrEscapesRoot) {
			return nil, err
		}
		return nil, fmt.Errorf("stat plans directory: %w", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []*Plan{}, nil
		}
		return nil, fmt.Errorf("stat plans directory: %w", err)
	}
	if !info.IsDir() {
		return []*Plan{}, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read plans directory: %w", err)
	}

	var plans []*Plan
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		planID := strings.TrimSuffix(name, ".md")
		plan, err := loadPlanFile(dir, planID, name)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}

	sort.Slice(plans, func(i, j int) bool {
		return plans[i].ID < plans[j].ID
	})

	return plans, nil
}

// Get loads and parses a single plan by ID from <projectRoot>/docs/ao/plans/<planID>.md.
func Get(projectRoot, planID string) (*Plan, error) {
	if !ValidateID(planID) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidID, planID)
	}

	dir, err := resolvePlansDir(projectRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %q", ErrNotFound, planID)
		}
		if errors.Is(err, ErrEscapesRoot) {
			return nil, err
		}
		return nil, fmt.Errorf("eval dir symlinks: %w", err)
	}

	fileName := planID + ".md"

	return loadPlanFile(dir, planID, fileName)
}

// loadPlanFile safely validates path confinement, bounds file size, checks slug matching,
// and parses the plan.
func loadPlanFile(dir, expectedID, fileName string) (*Plan, error) {
	if !ValidateID(expectedID) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidID, expectedID)
	}

	expectedFileName := expectedID + ".md"
	if fileName != expectedFileName {
		return nil, fmt.Errorf("%w: file name %q does not match plan id %q", ErrMalformed, fileName, expectedID)
	}

	// Ensure dir itself can be resolved
	cleanDir := filepath.Clean(dir)
	realDir, err := filepath.EvalSymlinks(cleanDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %q", ErrNotFound, expectedID)
		}
		return nil, fmt.Errorf("eval dir symlinks: %w", err)
	}

	targetPath := filepath.Join(cleanDir, fileName)

	// Ensure targetPath is syntactically within cleanDir
	rel, err := filepath.Rel(cleanDir, targetPath)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return nil, fmt.Errorf("%w: path %q traverses outside directory %q", ErrEscapesRoot, targetPath, cleanDir)
	}

	// Stat without following symlinks to check symlink status or existence
	if _, err := os.Lstat(targetPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %q", ErrNotFound, expectedID)
		}
		return nil, fmt.Errorf("lstat plan file: %w", err)
	}

	// Resolve symlinks to target file and ensure it stays within realDir
	realFile, err := filepath.EvalSymlinks(targetPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %q", ErrNotFound, expectedID)
		}
		return nil, fmt.Errorf("eval file symlinks: %w", err)
	}

	relToDir, err := filepath.Rel(realDir, realFile)
	if err != nil || strings.HasPrefix(relToDir, "..") || filepath.IsAbs(relToDir) {
		return nil, fmt.Errorf("%w: target %q escapes directory %q", ErrEscapesRoot, realFile, realDir)
	}

	// Check if target is a regular file
	fi, err := os.Stat(targetPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %q", ErrNotFound, expectedID)
		}
		return nil, fmt.Errorf("stat plan file: %w", err)
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("%w: target is a directory", ErrMalformed)
	}

	// Bound file size
	if fi.Size() > MaxPlanFileSize {
		return nil, fmt.Errorf("%w: size %d exceeds limit %d", ErrPlanTooLarge, fi.Size(), MaxPlanFileSize)
	}

	// Open file safely
	f, err := os.Open(targetPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %q", ErrNotFound, expectedID)
		}
		return nil, fmt.Errorf("open plan file: %w", err)
	}
	defer f.Close()

	// Read bounded bytes
	limitedReader := io.LimitReader(f, MaxPlanFileSize+1)
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("read plan file: %w", err)
	}
	if int64(len(content)) > MaxPlanFileSize {
		return nil, fmt.Errorf("%w: content exceeds limit %d", ErrPlanTooLarge, MaxPlanFileSize)
	}

	// Parse plan
	plan, err := Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}

	// Validate plan.ID slug matches expectedID and filename
	if plan.ID != expectedID {
		return nil, fmt.Errorf("%w: plan frontmatter id %q does not match filename slug %q", ErrMalformed, plan.ID, expectedID)
	}

	return plan, nil
}
