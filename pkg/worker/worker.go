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

// StateChangeCallback defines the type of state change callback function
type StateChangeCallback func(j *job.Job, status job.Status, message string, exitCode int, pid int)

// Worker responsible for executing Linux commands
type Worker struct {
	id            string
	onStateChange StateChangeCallback
}

// NewWorker creates a new Worker
func NewWorker(id string) *Worker {
	return &Worker{
		id:            id,
		onStateChange: nil,
	}
}

func (w *Worker) SetStateChangeCallback(callback StateChangeCallback) {
	w.onStateChange = callback
}

// ExecuteJob synchronously executes a job
func (w *Worker) ExecuteJob(j *job.Job) {
	// notify state change to running
	if w.onStateChange != nil {
		w.onStateChange(j, job.StatusRunning, "", 0, -1)
	}

	// Create a context that can be canceled
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(j.Timeout)*time.Second)
	defer cancel()

	// Prepare command
	cmdParts := strings.Fields(j.Command)
	if len(cmdParts) == 0 {
		// notify empty command error
		if w.onStateChange != nil {
			w.onStateChange(j, job.StatusFailed, "Empty command", 1, -1)
		}
		return
	}

	cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start command (instead of Run)
	if err := cmd.Start(); err != nil {
		// notify start failed
		if w.onStateChange != nil {
			w.onStateChange(j, job.StatusFailed, fmt.Sprintf("Failed to start command: %v", err), 1, -1)
		}
		return
	}

	// notify PID
	if w.onStateChange != nil {
		w.onStateChange(j, job.StatusRunning, "", 0, cmd.Process.Pid)
	}

	// Wait for the command to complete
	err := cmd.Wait()

	// Handle timeout
	if ctx.Err() == context.DeadlineExceeded {
		// notify timeout
		if w.onStateChange != nil {
			w.onStateChange(j, job.StatusFailed, fmt.Sprintf("Command timed out after %d seconds", j.Timeout), -1, -1)
		}
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

		// check if the error is due to SIGTERM (process was manually terminated)
		isSigterm := strings.Contains(err.Error(), "signal: terminated")

		if isSigterm && j.Status == job.StatusCancelled {
			// if already marked as cancelled, do nothing
			return
		} else if isSigterm {
			// if process was terminated by signal but not marked as cancelled, notify cancelled status
			if w.onStateChange != nil {
				w.onStateChange(j, job.StatusCancelled, stderr.String(), exitCode, -1)
			}
		} else {
			// other errors, notify failed status
			if w.onStateChange != nil {
				w.onStateChange(j, job.StatusFailed, fmt.Sprintf("Error: %v\nStderr: %s", err, stderr.String()), exitCode, -1)
			}
		}
		return
	}

	// notify completed status
	if w.onStateChange != nil {
		w.onStateChange(j, job.StatusCompleted, stdout.String(), 0, -1)
	}
}
