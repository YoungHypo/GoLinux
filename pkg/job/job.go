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
	ID          string
	Command     string
	Status      Status
	Result      string
	ErrorMsg    string
	CreatedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
	ExitCode    int
	Timeout     int
}

// NewJob creates a new job
func NewJob(command string, timeout int) *Job {
	return &Job{
		ID:        uuid.New().String(),
		Command:   command,
		Status:    StatusPending,
		CreatedAt: time.Now(),
		Timeout:   timeout,
	}
}

// SetRunning sets job status to running
func (j *Job) SetRunning() {
	j.Status = StatusRunning
	j.StartedAt = time.Now()
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
