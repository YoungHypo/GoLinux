package manager

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"

	"GoLinux/pkg/job"
	"GoLinux/pkg/storage"
	"GoLinux/pkg/worker"
)

type JobManager struct {
	jobs        map[string]*job.Job // In-memory cache for active jobs
	worker      *worker.Worker
	maxJobs     int
	storage     *storage.SQLiteStorage            // SQLite storage backend
	outputChans map[string]chan worker.OutputLine // Output channels for streaming jobs
	chanMutex   sync.RWMutex                      // Protects outputChans map
	eventChan   chan worker.JobEvent              // Channel for receiving job events
	stopChan    chan struct{}                     // Channel to stop event processing
	wg          sync.WaitGroup                    // WaitGroup for goroutines
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

	eventChan := make(chan worker.JobEvent, 1000) // Buffered channel for events

	jm := &JobManager{
		jobs:        make(map[string]*job.Job),
		worker:      worker.NewWorker("worker-1"),
		maxJobs:     maxJobs,
		storage:     store,
		outputChans: make(map[string]chan worker.OutputLine),
		eventChan:   eventChan,
		stopChan:    make(chan struct{}),
	}

	// Set up event channel for the worker
	jm.worker.SetEventChannel(eventChan)

	// Start event processing goroutine
	jm.wg.Add(1)
	go jm.processEvents()

	// Load active jobs into memory cache
	if err := jm.loadActiveJobs(); err != nil {
		store.Close()
		return nil, fmt.Errorf("failed to load active jobs: %v", err)
	}

	return jm, nil
}

// Close closes the JobManager and its storage
func (jm *JobManager) Close() error {
	// Stop event processing
	close(jm.stopChan)
	jm.wg.Wait()

	// Close event channel
	close(jm.eventChan)

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

	// Create output channel for streaming
	outputChan := make(chan worker.OutputLine, 100) // Buffered channel
	jm.chanMutex.Lock()
	jm.outputChans[j.ID] = outputChan
	jm.chanMutex.Unlock()

	// Execute job asynchronously with streaming
	go jm.ExecuteJob(j, outputChan)

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

// executeJob executes a job with streaming output
func (jm *JobManager) ExecuteJob(j *job.Job, outputChan chan worker.OutputLine) {
	defer func() {
		// Clean up channel when job completes
		jm.chanMutex.Lock()
		delete(jm.outputChans, j.ID)
		jm.chanMutex.Unlock()
	}()

	// Execute job with channel, all state changes will be handled through callback
	jm.worker.ExecuteJob(j, outputChan)
}

// GetOutputChannel returns the output channel for a job
func (jm *JobManager) GetOutputChannel(jobID string) <-chan worker.OutputLine {
	jm.chanMutex.RLock()
	defer jm.chanMutex.RUnlock()

	if ch, exists := jm.outputChans[jobID]; exists {
		return ch
	}
	return nil
}

// loadActiveJobs cancels old jobs from previous server sessions
func (jm *JobManager) loadActiveJobs() error {
	pendingJobs, err := jm.storage.ListJobs(job.StatusPending)
	if err != nil {
		return fmt.Errorf("failed to load pending jobs: %v", err)
	}

	runningJobs, err := jm.storage.ListJobs(job.StatusRunning)
	if err != nil {
		return fmt.Errorf("failed to load running jobs: %v", err)
	}

	// Cancel all old jobs from previous server sessions
	for _, j := range pendingJobs {
		j.SetCancelled()
		if err := jm.storage.SaveJob(j); err != nil {
			fmt.Printf("Warning: failed to cancel pending job %s: %v\n", j.ID, err)
		}
	}

	for _, j := range runningJobs {
		j.SetCancelled()
		if err := jm.storage.SaveJob(j); err != nil {
			fmt.Printf("Warning: failed to cancel running job %s: %v\n", j.ID, err)
		}
	}

	return nil
}

// processEvents processes job events from the event channel
func (jm *JobManager) processEvents() {
	defer jm.wg.Done()

	for {
		select {
		case event := <-jm.eventChan:
			jm.handleJobEvent(event)
		case <-jm.stopChan:
			return
		}
	}
}

// handleJobEvent handles a single job event
func (jm *JobManager) handleJobEvent(event worker.JobEvent) {
	j, exists := jm.jobs[event.JobID]
	if !exists {
		// Try to load from database
		var err error
		j, err = jm.storage.GetJob(event.JobID)
		if err != nil {
			fmt.Printf("Warning: job %s not found: %v\n", event.JobID, err)
			return
		}
		// Add to memory cache if it's an active job
		if !j.IsComplete() {
			jm.jobs[j.ID] = j
		}
	}

	// Handle different event types
	switch event.Type {
	case worker.EventJobStarted:
		if j.Status == job.StatusPending {
			j.SetRunning()
		}
	case worker.EventJobPIDAssigned:
		if event.PID > 0 {
			j.SetPid(event.PID)
		}
	case worker.EventJobOutput:
		if event.Message != "" {
			j.Result = event.Message
		}
	case worker.EventJobCompleted:
		j.SetCompleted(event.Message, event.ExitCode)
		delete(jm.jobs, j.ID)
	case worker.EventJobFailed, worker.EventJobTimeout:
		j.SetFailed(event.Message, event.ExitCode)
		delete(jm.jobs, j.ID)
	case worker.EventJobCancelled:
		j.SetCancelled()
		delete(jm.jobs, j.ID)
	}

	// Save updated job state to database
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
