// Package container manages Docker containers for janus worktrees.
package container

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ContainerName returns the Docker container name for the given worktree id.
func ContainerName(repoID, worktreeID string) string {
	return fmt.Sprintf("janus-%s-%s", repoID, worktreeID)
}

// Start creates and starts a new container for the workspace.
// merged is the host path of the overlay MERGED directory.
func Start(name, image, merged string) error {
	cmd := exec.Command(
		"docker", "run", "-d",
		"--name", name,
		"-v", merged+":/workspace",
		"-w", "/workspace",
		image,
		"sleep", "infinity",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("starting container %s: %w", name, err)
	}
	return nil
}

// Stop stops and removes the container with the given name.
func Stop(name string) error {
	// stop (ignore error if already stopped)
	exec.Command("docker", "stop", name).Run() //nolint:errcheck
	// remove
	cmd := exec.Command("docker", "rm", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("removing container %s: %w", name, err)
	}
	return nil
}

// IsRunning reports whether a container with the given name is running.
func IsRunning(name string) (bool, error) {
	out, err := exec.Command(
		"docker", "inspect", "--format", "{{.State.Running}}", name,
	).Output()
	if err != nil {
		// container does not exist → not running
		return false, nil
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

// EnsureRunning starts the container if it is not already running.
// If the container does not exist yet it is created.
func EnsureRunning(name, image, merged string) error {
	running, err := IsRunning(name)
	if err != nil {
		return err
	}
	if running {
		return nil
	}
	// Container may exist but be stopped — try starting it first.
	startCmd := exec.Command("docker", "start", name)
	var startErr bytes.Buffer
	startCmd.Stderr = &startErr
	if startCmd.Run() == nil {
		return nil
	}
	// Container does not exist — create it.
	return Start(name, image, merged)
}

// Enter opens an interactive bash shell in the given container.
func Enter(name string) error {
	cmd := exec.Command("docker", "exec", "-it", name, "bash")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Exec runs a non-interactive command in the given container.
func Exec(name string, args []string) error {
	fullArgs := append([]string{"exec", name}, args...)
	cmd := exec.Command("docker", fullArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
