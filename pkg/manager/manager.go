package manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"GoLinux/pkg/job"
	"GoLinux/pkg/worker"
)

type JobManager struct {
	jobs      map[string]*job.Job // Stores mapping from job ID to job object
	worker    *worker.Worker
	maxJobs   int
	storePath string // Path to store job files
}

// NewJobManager creates a new JobManager
func NewJobManager(maxJobs int) *JobManager {
	if maxJobs <= 0 {
		maxJobs = 1000 // Default value
	}

	// Create storage directory
	storePath := "jobs"
	os.MkdirAll(storePath, 0755)

	jm := &JobManager{
		jobs:      make(map[string]*job.Job),
		worker:    worker.NewWorker("worker-1"),
		maxJobs:   maxJobs,
		storePath: storePath,
	}

	// Load existing jobs
	jm.loadJobs()

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

	// Save job to persistent storage
	jm.saveJob(j)

	jm.executeJob(j)

	return j, nil
}

// GetJob retrieves a Job by ID
func (jm *JobManager) GetJob(jobID string) (*job.Job, error) {
	j, exists := jm.jobs[jobID]
	if exists {
		return j, nil
	}

	// If not in memory, try to load from file
	j, err := jm.loadJobFromFile(jobID)
	if err != nil {
		return nil, fmt.Errorf("job with ID %s not found: %v", jobID, err)
	}

	// Add to memory cache
	jm.jobs[j.ID] = j

	return j, nil
}

// ListJobs lists all Jobs with optional status filter
func (jm *JobManager) ListJobs(status job.Status) []*job.Job {
	// Load all jobs from storage before listing
	jm.loadJobs()

	var result []*job.Job

	for _, j := range jm.jobs {
		if status == "" || j.Status == status {
			result = append(result, j)
		}
	}

	return result
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

func (jm *JobManager) CleanJobs() (int, error) {
	// First count the number of jobs to delete
	entries, err := os.ReadDir(jm.storePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read jobs directory: %v", err)
	}

	count := 0

	// Delete all JSON files in the job directory
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			filePath := filepath.Join(jm.storePath, entry.Name())
			if err := os.Remove(filePath); err != nil {
				fmt.Printf("Warning: failed to delete %s: %v\n", filePath, err)
				continue
			}
			count++
		}
	}

	// Clear the in-memory job cache
	jm.jobs = make(map[string]*job.Job)

	return count, nil
}

// executeJob executes a Job
func (jm *JobManager) executeJob(j *job.Job) {
	jm.worker.ExecuteJob(j)

	// Save job after execution is complete
	jm.saveJob(j)
}

// Save job to file
func (jm *JobManager) saveJob(j *job.Job) error {
	data, err := json.Marshal(j)
	if err != nil {
		return fmt.Errorf("error marshaling job: %v", err)
	}

	filename := filepath.Join(jm.storePath, j.ID+".json")
	return os.WriteFile(filename, data, 0644)
}

// Load a single job from file
func (jm *JobManager) loadJobFromFile(jobID string) (*job.Job, error) {
	filename := filepath.Join(jm.storePath, jobID+".json")
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var j job.Job
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, err
	}

	return &j, nil
}

// Load all jobs from files
func (jm *JobManager) loadJobs() error {
	entries, err := os.ReadDir(jm.storePath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		jobID := entry.Name()[:len(entry.Name())-5] // Remove .json extension
		j, err := jm.loadJobFromFile(jobID)
		if err != nil {
			continue // Skip files that can't be loaded
		}

		// Add to memory cache if not already present
		if _, exists := jm.jobs[j.ID]; !exists {
			jm.jobs[j.ID] = j
		}
	}

	return nil
}
