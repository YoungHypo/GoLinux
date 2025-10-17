package worker

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"GoLinux/pkg/job"
)

// OutputLine represents a line of output with metadata
type OutputLine struct {
	Content   string
	IsStderr  bool
	Timestamp time.Time
}

// JobEvent represents different types of job events
type JobEvent struct {
	JobID     string
	Type      JobEventType
	Status    job.Status
	Message   string
	ExitCode  int
	PID       int
	Timestamp time.Time
}

// JobEventType defines the type of job event
type JobEventType int

const (
	EventJobStarted JobEventType = iota
	EventJobOutput
	EventJobPIDAssigned
	EventJobCompleted
	EventJobFailed
	EventJobCancelled
	EventJobTimeout
)

// Worker responsible for executing Linux commands
type Worker struct {
	id        string
	eventChan chan<- JobEvent // Channel for sending job events
}

// NewWorker creates a new Worker
func NewWorker(id string) *Worker {
	return &Worker{
		id: id,
	}
}

// SetEventChannel sets the event channel for the worker
func (w *Worker) SetEventChannel(eventChan chan<- JobEvent) {
	w.eventChan = eventChan
}

// sendEvent sends a job event through the channel (non-blocking)
func (w *Worker) sendEvent(jobID string, eventType JobEventType, status job.Status, message string, exitCode, pid int) {
	if w.eventChan != nil {
		event := JobEvent{
			JobID:     jobID,
			Type:      eventType,
			Status:    status,
			Message:   message,
			ExitCode:  exitCode,
			PID:       pid,
			Timestamp: time.Now(),
		}

		select {
		case w.eventChan <- event:
		default:
			// Channel full, continue without blocking
		}
	}
}

func (w *Worker) parseCommand(command string) (string, []string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", nil, fmt.Errorf("empty command")
	}

	// Handle shell commands like "bash -c", "sh -c", etc.
	shellPattern := regexp.MustCompile(`^(bash|sh|zsh|fish)\s+-c\s+(.+)$`)
	if matches := shellPattern.FindStringSubmatch(command); len(matches) == 3 {
		shell := matches[1]
		shellCmd := strings.TrimSpace(matches[2])

		// Remove outer quotes if present (but preserve inner content)
		if (strings.HasPrefix(shellCmd, `"`) && strings.HasSuffix(shellCmd, `"`)) ||
			(strings.HasPrefix(shellCmd, `'`) && strings.HasSuffix(shellCmd, `'`)) {
			shellCmd = shellCmd[1 : len(shellCmd)-1]
		}

		return shell, []string{"-c", shellCmd}, nil
	}

	// Handle regular commands
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("empty command")
	}

	return parts[0], parts[1:], nil
}

// ExecuteJob executes a job and optionally streams output to a channel
func (w *Worker) ExecuteJob(j *job.Job, outputChan chan<- OutputLine) {
	// Send job started event
	w.sendEvent(j.ID, EventJobStarted, job.StatusRunning, "", 0, -1)

	// Create a context that can be canceled
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(j.Timeout)*time.Second)
	defer cancel()

	cmdName, cmdArgs, err := w.parseCommand(j.Command)
	if err != nil {
		// Send parsing error event
		w.sendEvent(j.ID, EventJobFailed, job.StatusFailed, fmt.Sprintf("Command parsing error: %v", err), 1, -1)
		return
	}

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)

	// Set up pipes for real-time output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		w.sendEvent(j.ID, EventJobFailed, job.StatusFailed, fmt.Sprintf("Failed to create stdout pipe: %v", err), 1, -1)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		w.sendEvent(j.ID, EventJobFailed, job.StatusFailed, fmt.Sprintf("Failed to create stderr pipe: %v", err), 1, -1)
		return
	}

	// Start command
	if err := cmd.Start(); err != nil {
		w.sendEvent(j.ID, EventJobFailed, job.StatusFailed, fmt.Sprintf("Failed to start command: %v", err), 1, -1)
		return
	}

	// Send PID assigned event
	w.sendEvent(j.ID, EventJobPIDAssigned, job.StatusRunning, "", 0, cmd.Process.Pid)

	// Collect output in real-time
	var outputBuffer bytes.Buffer
	var outputMutex sync.Mutex
	var wg sync.WaitGroup

	// Read stdout in a separate goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			outputMutex.Lock()
			outputBuffer.WriteString(line)
			currentOutput := outputBuffer.String()
			outputMutex.Unlock()

			// Send to channel if available (non-blocking)
			if outputChan != nil {
				select {
				case outputChan <- OutputLine{
					Content:   line,
					IsStderr:  false,
					Timestamp: time.Now(),
				}:
				default:
					// Channel full, continue without blocking
				}
			}

			// Send output event
			w.sendEvent(j.ID, EventJobOutput, job.StatusRunning, currentOutput, 0, -1)
		}
	}()

	// Read stderr in a separate goroutine
	var stderrBuffer bytes.Buffer
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			stderrBuffer.WriteString(line)

			// Send stderr to channel if available (non-blocking)
			if outputChan != nil {
				select {
				case outputChan <- OutputLine{
					Content:   line,
					IsStderr:  true,
					Timestamp: time.Now(),
				}:
				default:
					// Channel full, continue without blocking
				}
			}
		}
	}()

	// Wait for the command to complete
	err = cmd.Wait()

	// Wait for output readers to finish
	wg.Wait()

	// Get final output
	outputMutex.Lock()
	finalOutput := outputBuffer.String()
	outputMutex.Unlock()
	finalStderr := stderrBuffer.String()

	// Handle timeout
	if ctx.Err() == context.DeadlineExceeded {
		// Close output channel on timeout
		if outputChan != nil {
			close(outputChan)
		}
		// Send timeout event
		w.sendEvent(j.ID, EventJobTimeout, job.StatusFailed, fmt.Sprintf("Command timed out after %d seconds", j.Timeout), -1, -1)
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
			// Send cancelled event
			w.sendEvent(j.ID, EventJobCancelled, job.StatusCancelled, finalStderr, exitCode, -1)
		} else {
			// Send failed event
			w.sendEvent(j.ID, EventJobFailed, job.StatusFailed, fmt.Sprintf("Error: %v\nStderr: %s", err, finalStderr), exitCode, -1)
		}
		// Close output channel on error
		if outputChan != nil {
			close(outputChan)
		}
		return
	}

	// Close output channel to signal completion
	if outputChan != nil {
		close(outputChan)
	}

	// Send completed event
	w.sendEvent(j.ID, EventJobCompleted, job.StatusCompleted, finalOutput, 0, -1)
}
