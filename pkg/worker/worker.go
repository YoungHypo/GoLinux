package worker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"GoLinux/pkg/job"
)

// Worker responsible for executing Linux commands
type Worker struct {
	id string
}

// NewWorker creates a new Worker
func NewWorker(id string) *Worker {
	return &Worker{
		id: id,
	}
}

// ExecuteJob synchronously executes a job
func (w *Worker) ExecuteJob(j *job.Job) {
	j.SetRunning()

	// Create a context that can be canceled
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(j.Timeout)*time.Second)
	defer cancel()

	// Prepare command
	cmdParts := strings.Fields(j.Command)
	if len(cmdParts) == 0 {
		j.SetFailed("Empty command", 1)
		return
	}

	cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute command
	err := cmd.Run()

	// Handle timeout
	if ctx.Err() == context.DeadlineExceeded {
		j.SetFailed(fmt.Sprintf("Command timed out after %d seconds", j.Timeout), -1)
		return
	}

	// Handle errors
	if err != nil {
		var exitCode int
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
		j.SetFailed(fmt.Sprintf("Error: %v\nStderr: %s", err, stderr.String()), exitCode)
		return
	}

	// Set job as completed
	j.SetCompleted(stdout.String(), 0)
}
