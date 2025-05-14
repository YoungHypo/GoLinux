package job

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending   Status = "PENDING"
	StatusRunning   Status = "RUNNING"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
	StatusCancelled Status = "CANCELLED"
)

type Job struct {
	ID          string    `json:"id"`
	Command     string    `json:"command"`
	Status      Status    `json:"status"`
	Result      string    `json:"result"`
	ErrorMsg    string    `json:"error_message"`
	CreatedAt   time.Time `json:"created_at"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	ExitCode    int       `json:"exit_code"`
	Timeout     int       `json:"timeout"`
	Pid         int       `json:"pid"`
}

// NewJob creates a new job
func NewJob(command string, timeout int) *Job {
	return &Job{
		ID:        uuid.New().String(),
		Command:   command,
		Status:    StatusPending,
		CreatedAt: time.Now(),
		Timeout:   timeout,
		Pid:       -1, // Initialize with invalid PID
	}
}

// SetRunning sets job status to running
func (j *Job) SetRunning() {
	j.Status = StatusRunning
	j.StartedAt = time.Now()
}

func (j *Job) SetPid(pid int) {
	j.Pid = pid
}

// SetCompleted sets job status to completed
func (j *Job) SetCompleted(result string, exitCode int) {
	j.Status = StatusCompleted
	j.Result = result
	j.ExitCode = exitCode
	j.CompletedAt = time.Now()
}

// SetFailed sets job status to failed
func (j *Job) SetFailed(errorMsg string, exitCode int) {
	j.Status = StatusFailed
	j.ErrorMsg = errorMsg
	j.ExitCode = exitCode
	j.CompletedAt = time.Now()
}

// SetCancelled sets job status to cancelled
func (j *Job) SetCancelled() {
	j.Status = StatusCancelled
	j.CompletedAt = time.Now()
}

// IsComplete checks if the job is complete (regardless of success or failure)
func (j *Job) IsComplete() bool {
	return j.Status == StatusCompleted || j.Status == StatusFailed || j.Status == StatusCancelled
}

// IsRunning checks if the job is currently running
func (j *Job) IsRunning() bool {
	return j.Status == StatusRunning
}

// Duration returns the job execution duration
func (j *Job) Duration() time.Duration {
	if j.IsComplete() && !j.CompletedAt.IsZero() && !j.StartedAt.IsZero() {
		return j.CompletedAt.Sub(j.StartedAt)
	}
	if !j.StartedAt.IsZero() {
		return time.Since(j.StartedAt)
	}
	return 0
}
