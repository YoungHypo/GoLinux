package manager

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"GoLinux/pkg/job"
	"GoLinux/pkg/storage"
	"GoLinux/pkg/worker"
)

type JobManager struct {
	jobs    map[string]*job.Job // In-memory cache for active jobs
	worker  *worker.Worker
	maxJobs int
	storage *storage.SQLiteStorage // SQLite storage backend
}

// NewJobManager creates a new JobManager
func NewJobManager(maxJobs int, dbPath string) (*JobManager, error) {
	if maxJobs <= 0 {
		maxJobs = 1000 // Default value
	}

	if dbPath == "" {
		dbPath = "jobs.db" // Default database path
	}

	// Initialize SQLite storage
	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %v", err)
	}

	jm := &JobManager{
		jobs:    make(map[string]*job.Job),
		worker:  worker.NewWorker("worker-1"),
		maxJobs: maxJobs,
		storage: store,
	}

	// set state change callback
	jm.worker.SetStateChangeCallback(jm.handleStateChange)

	// Load active jobs into memory cache
	if err := jm.loadActiveJobs(); err != nil {
		store.Close()
		return nil, fmt.Errorf("failed to load active jobs: %v", err)
	}

	return jm, nil
}

// Close closes the JobManager and its storage
func (jm *JobManager) Close() error {
	return jm.storage.Close()
}

// CreateJob creates and runs a new Job
func (jm *JobManager) CreateJob(command string, timeoutSeconds int) (*job.Job, error) {
	// Check total job count from database instead of just memory cache
	totalJobs, err := jm.storage.GetJobCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get job count: %v", err)
	}

	if totalJobs >= jm.maxJobs {
		return nil, errors.New("maximum number of jobs reached")
	}

	if command == "" {
		return nil, errors.New("command cannot be empty")
	}

	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}

	j := job.NewJob(command, timeoutSeconds)

	if err := jm.storage.SaveJob(j); err != nil {
		return nil, fmt.Errorf("failed to save job: %v", err)
	}

	jm.jobs[j.ID] = j

	// Execute job asynchronously
	go jm.executeJob(j)

	return j, nil
}

// GetJob retrieves a Job by ID
func (jm *JobManager) GetJob(jobID string) (*job.Job, error) {
	// Check memory cache first
	if j, exists := jm.jobs[jobID]; exists {
		return j, nil
	}

	// If not in memory, load from database
	j, err := jm.storage.GetJob(jobID)
	if err != nil {
		return nil, err
	}

	// Add to memory cache if it's an active job
	if !j.IsComplete() {
		jm.jobs[j.ID] = j
	}

	return j, nil
}

// ListJobs lists all Jobs with optional status filter
func (jm *JobManager) ListJobs(status job.Status) ([]*job.Job, error) {
	jobs, err := jm.storage.ListJobs(status)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %v", err)
	}

	return jobs, nil
}

func (jm *JobManager) CancelJob(jobID string) error {
	// Try to get job from memory first, then from database
	j, err := jm.GetJob(jobID)
	if err != nil {
		return err
	}

	if j.IsComplete() {
		return fmt.Errorf("job is already complete with status %s", j.Status)
	}

	j.SetCancelled()

	if err := jm.storage.SaveJob(j); err != nil {
		return fmt.Errorf("failed to save job status: %v", err)
	}

	return nil
}

func (jm *JobManager) CleanJobs() (int64, error) {
	// Delete all jobs from database
	count, err := jm.storage.DeleteAllJobs()
	if err != nil {
		return 0, fmt.Errorf("failed to delete all jobs: %v", err)
	}

	// Clear the in-memory job cache
	jm.jobs = make(map[string]*job.Job)

	return count, nil
}

// executeJob executes a Job
func (jm *JobManager) executeJob(j *job.Job) {
	// execute job, all state changes will be handled through callback
	jm.worker.ExecuteJob(j)
}

// loadActiveJobs loads running and pending jobs into memory cache
func (jm *JobManager) loadActiveJobs() error {
	pendingJobs, err := jm.storage.ListJobs(job.StatusPending)
	if err != nil {
		return fmt.Errorf("failed to load pending jobs: %v", err)
	}

	runningJobs, err := jm.storage.ListJobs(job.StatusRunning)
	if err != nil {
		return fmt.Errorf("failed to load running jobs: %v", err)
	}

	// Add to memory cache
	for _, j := range pendingJobs {
		jm.jobs[j.ID] = j
	}
	for _, j := range runningJobs {
		jm.jobs[j.ID] = j
	}

	return nil
}

func (jm *JobManager) handleStateChange(j *job.Job, status job.Status, message string, exitCode int, pid int) {
	// update job state based on status
	switch status {
	case job.StatusRunning:
		j.SetRunning()
		if pid > 0 {
			j.SetPid(pid)
		}
	case job.StatusCompleted:
		j.SetCompleted(message, exitCode)
		delete(jm.jobs, j.ID)
	case job.StatusFailed:
		j.SetFailed(message, exitCode)
		delete(jm.jobs, j.ID)
	case job.StatusCancelled:
		j.SetCancelled()
		delete(jm.jobs, j.ID)
	}

	// save updated job state to database
	if err := jm.storage.SaveJob(j); err != nil {
		fmt.Printf("Warning: failed to save job %s: %v\n", j.ID, err)
	}
}

// StopJob stops a running job by its ID
func (jm *JobManager) StopJob(jobID string) error {
	// Get job from memory or database
	j, err := jm.GetJob(jobID)
	if err != nil {
		return err
	}

	if !j.IsRunning() {
		return fmt.Errorf("job with ID %s is not running, current status: %s", jobID, j.Status)
	}

	if j.Pid <= 0 {
		return fmt.Errorf("cannot stop job %s: invalid process ID (%d)", jobID, j.Pid)
	}

	// mark job as cancelled
	j.SetCancelled()
	if err := jm.storage.SaveJob(j); err != nil {
		return fmt.Errorf("failed to save job status: %v", err)
	}

	// Send SIGTERM to the process
	process, err := os.FindProcess(j.Pid)
	if err != nil {
		return fmt.Errorf("failed to find process with PID %d: %v", j.Pid, err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		// if failed to send SIGTERM, restore job state
		j.Status = job.StatusRunning
		if saveErr := jm.storage.SaveJob(j); saveErr != nil {
			fmt.Printf("Warning: failed to restore job status: %v\n", saveErr)
		}
		return fmt.Errorf("failed to send SIGTERM to process with PID %d: %v", j.Pid, err)
	}

	return nil
}
