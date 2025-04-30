package manager

import (
	"errors"
	"fmt"

	"GoLinux/pkg/job"
	"GoLinux/pkg/worker"
)

type JobManager struct {
	jobs    map[string]*job.Job // Stores mapping from job ID to job object
	worker  *worker.Worker
	maxJobs int
}

// NewJobManager creates a new JobManager
func NewJobManager(maxJobs int) *JobManager {
	if maxJobs <= 0 {
		maxJobs = 1000 // Default value
	}

	jm := &JobManager{
		jobs:    make(map[string]*job.Job),
		worker:  worker.NewWorker("worker-1"),
		maxJobs: maxJobs,
	}

	return jm
}

// CreateJob creates and runs a new Job
func (jm *JobManager) CreateJob(command string, timeoutSeconds int) (*job.Job, error) {
	if len(jm.jobs) >= jm.maxJobs {
		return nil, errors.New("maximum number of jobs reached")
	}

	if command == "" {
		return nil, errors.New("command cannot be empty")
	}

	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}

	j := job.NewJob(command, timeoutSeconds)
	jm.jobs[j.ID] = j

	jm.executeJob(j)

	return j, nil
}

// GetJob retrieves a Job by ID
func (jm *JobManager) GetJob(jobID string) (*job.Job, error) {
	j, exists := jm.jobs[jobID]
	if exists {
		return j, nil
	}

	return nil, fmt.Errorf("job with ID %s not found", jobID)
}

func (jm *JobManager) CancelJob(jobID string) error {
	j, exists := jm.jobs[jobID]
	if !exists {
		return fmt.Errorf("job with ID %s not found", jobID)
	}

	if j.IsComplete() {
		return fmt.Errorf("job is already complete with status %s", j.Status)
	}

	j.SetCancelled()
	return nil
}

// executeJob executes a Job
func (jm *JobManager) executeJob(j *job.Job) {
	jm.worker.ExecuteJob(j)
}
