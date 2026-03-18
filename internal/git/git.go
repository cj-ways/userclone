package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Result struct {
	Name   string
	Status string // "cloned", "updated", "up to date", "skipped", "failed"
	Error  error
}

// Exists checks whether a repo directory already exists at the destination.
func Exists(dest string, repoName string) bool {
	repoPath := filepath.Join(dest, repoName)
	info, err := os.Stat(repoPath)
	return err == nil && info.IsDir()
}

func CloneOrPull(cloneURL string, dest string, repoName string) Result {
	repoPath := filepath.Join(dest, repoName)

	info, statErr := os.Stat(repoPath)

	// Path exists as a file (not a directory) — cannot clone here
	if statErr == nil && !info.IsDir() {
		return Result{
			Name:   repoName,
			Status: "skipped",
			Error:  fmt.Errorf("path exists as a file, not a directory"),
		}
	}

	// Directory exists
	if statErr == nil && info.IsDir() {
		gitDir := filepath.Join(repoPath, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			// Directory exists but is not a git repo — don't overwrite
			return Result{
				Name:   repoName,
				Status: "skipped",
				Error:  fmt.Errorf("directory exists but is not a git repo"),
			}
		}
		return pull(repoPath, repoName)
	}

	return clone(cloneURL, repoPath, repoName)
}

func clone(url string, dest string, name string) Result {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return Result{Name: name, Status: "failed", Error: fmt.Errorf("creating directory: %w", err)}
	}

	cmd := exec.Command("git", "clone", "--quiet", url, dest)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return Result{
			Name:   name,
			Status: "failed",
			Error:  fmt.Errorf("git clone: %s", strings.TrimSpace(string(output))),
		}
	}

	return Result{Name: name, Status: "cloned"}
}

func pull(repoPath string, name string) Result {
	// Check for detached HEAD — don't try to pull
	headCmd := exec.Command("git", "-C", repoPath, "symbolic-ref", "--quiet", "HEAD")
	if err := headCmd.Run(); err != nil {
		return Result{Name: name, Status: "skipped", Error: fmt.Errorf("detached HEAD")}
	}

	// Check for dirty working tree — don't risk losing uncommitted work
	dirtyCmd := exec.Command("git", "-C", repoPath, "status", "--porcelain")
	dirtyOutput, err := dirtyCmd.CombinedOutput()
	if err == nil && len(strings.TrimSpace(string(dirtyOutput))) > 0 {
		return Result{Name: name, Status: "skipped", Error: fmt.Errorf("has local changes")}
	}

	fetchCmd := exec.Command("git", "-C", repoPath, "fetch", "--quiet")
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		return Result{
			Name:   name,
			Status: "failed",
			Error:  fmt.Errorf("git fetch: %s", strings.TrimSpace(string(output))),
		}
	}

	statusCmd := exec.Command("git", "-C", repoPath, "status", "--porcelain", "--branch")
	output, err := statusCmd.CombinedOutput()
	if err != nil {
		return Result{Name: name, Status: "up to date"}
	}

	statusStr := string(output)
	if !strings.Contains(statusStr, "behind") {
		return Result{Name: name, Status: "up to date"}
	}

	pullCmd := exec.Command("git", "-C", repoPath, "pull", "--quiet", "--ff-only")
	pullOutput, err := pullCmd.CombinedOutput()
	if err != nil {
		return Result{
			Name:   name,
			Status: "failed",
			Error:  fmt.Errorf("git pull: %s", strings.TrimSpace(string(pullOutput))),
		}
	}

	return Result{Name: name, Status: "updated"}
}
