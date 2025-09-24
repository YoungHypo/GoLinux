package grpc

import (
	"context"
	"fmt"
	"time"

	"GoLinux/pkg/job"
	"GoLinux/pkg/manager"
	pb "GoLinux/proto"
)

type JobServiceServer struct {
	pb.UnimplementedJobServiceServer
	manager *manager.JobManager
}

func NewJobServiceServer(jm *manager.JobManager) *JobServiceServer {
	return &JobServiceServer{
		manager: jm,
	}
}

func (s *JobServiceServer) CreateJob(ctx context.Context, req *pb.CreateJobRequest) (*pb.CreateJobResponse, error) {
	if req.Command == "" {
		return &pb.CreateJobResponse{
			Success: false,
			Error:   "Command cannot be empty",
		}, nil
	}

	timeout := int(req.Timeout)
	if timeout <= 0 {
		timeout = 60 // Default 60 seconds
	}

	j, err := s.manager.CreateJob(req.Command, timeout)
	if err != nil {
		return &pb.CreateJobResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.CreateJobResponse{
		JobId:   j.ID,
		Success: true,
	}, nil
}

func (s *JobServiceServer) GetJobStatus(ctx context.Context, req *pb.GetJobStatusRequest) (*pb.JobStatusResponse, error) {
	if req.JobId == "" {
		return nil, fmt.Errorf("job ID cannot be empty")
	}

	j, err := s.manager.GetJob(req.JobId)
	if err != nil {
		return nil, err
	}

	return s.jobToResponse(j), nil
}

func (s *JobServiceServer) StopJob(ctx context.Context, req *pb.StopJobRequest) (*pb.StopJobResponse, error) {
	if req.JobId == "" {
		return &pb.StopJobResponse{
			Success: false,
			Error:   "Job ID cannot be empty",
		}, nil
	}

	err := s.manager.StopJob(req.JobId)
	if err != nil {
		return &pb.StopJobResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.StopJobResponse{
		Success: true,
	}, nil
}

func (s *JobServiceServer) ListJobs(ctx context.Context, req *pb.ListJobsRequest) (*pb.ListJobsResponse, error) {
	jobs, err := s.manager.ListJobs("")
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %v", err)
	}

	var pbJobs []*pb.Job
	for _, j := range jobs {
		pbJobs = append(pbJobs, s.jobToProtoJob(j))
	}

	return &pb.ListJobsResponse{
		Jobs:       pbJobs,
		TotalCount: int32(len(jobs)),
	}, nil
}

func (s *JobServiceServer) CleanJobs(ctx context.Context, req *pb.CleanJobsRequest) (*pb.CleanJobsResponse, error) {
	if !req.Confirm {
		return &pb.CleanJobsResponse{
			Success: false,
			Error:   "Confirmation required to clean jobs",
		}, nil
	}

	count, err := s.manager.CleanJobs()
	if err != nil {
		return &pb.CleanJobsResponse{
			Success:      false,
			Error:        err.Error(),
			DeletedCount: 0,
		}, nil
	}

	return &pb.CleanJobsResponse{
		Success:      true,
		DeletedCount: int32(count),
	}, nil
}

// StreamJobStatus streams job status updates
func (s *JobServiceServer) StreamJobStatus(req *pb.GetJobStatusRequest, stream pb.JobService_StreamJobStatusServer) error {
	if req.JobId == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	// Check if job exists
	j, err := s.manager.GetJob(req.JobId)
	if err != nil {
		return err
	}

	if err := stream.Send(s.jobToResponse(j)); err != nil {
		return err
	}

	if j.IsComplete() {
		return nil
	}

	// Poll for status updates every 500ms
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-ticker.C:
			j, err := s.manager.GetJob(req.JobId)
			if err != nil {
				return err
			}

			if err := stream.Send(s.jobToResponse(j)); err != nil {
				return err
			}

			// Stop streaming if job is complete
			if j.IsComplete() {
				return nil
			}
		}
	}
}

func (s *JobServiceServer) jobToResponse(j *job.Job) *pb.JobStatusResponse {
	var duration int64
	if !j.CompletedAt.IsZero() && !j.StartedAt.IsZero() {
		duration = j.CompletedAt.Sub(j.StartedAt).Milliseconds()
	} else if !j.StartedAt.IsZero() {
		duration = time.Since(j.StartedAt).Milliseconds()
	}

	return &pb.JobStatusResponse{
		JobId:        j.ID,
		Command:      j.Command,
		Status:       s.statusToProto(j.Status),
		Result:       j.Result,
		ErrorMessage: j.ErrorMsg,
		CreatedAt:    j.CreatedAt.Format(time.RFC3339),
		StartedAt:    j.StartedAt.Format(time.RFC3339),
		CompletedAt:  j.CompletedAt.Format(time.RFC3339),
		ExitCode:     int32(j.ExitCode),
		Pid:          int32(j.Pid),
		IsComplete:   j.IsComplete(),
		DurationMs:   duration,
	}
}

// Helper function to convert job.Job to pb.Job
func (s *JobServiceServer) jobToProtoJob(j *job.Job) *pb.Job {
	var duration int64
	if !j.CompletedAt.IsZero() && !j.StartedAt.IsZero() {
		duration = j.CompletedAt.Sub(j.StartedAt).Milliseconds()
	} else if !j.StartedAt.IsZero() {
		duration = time.Since(j.StartedAt).Milliseconds()
	}

	return &pb.Job{
		Id:           j.ID,
		Command:      j.Command,
		Status:       s.statusToProto(j.Status),
		Result:       j.Result,
		ErrorMessage: j.ErrorMsg,
		CreatedAt:    j.CreatedAt.Format(time.RFC3339),
		StartedAt:    j.StartedAt.Format(time.RFC3339),
		CompletedAt:  j.CompletedAt.Format(time.RFC3339),
		ExitCode:     int32(j.ExitCode),
		Timeout:      int32(j.Timeout),
		Pid:          int32(j.Pid),
		DurationMs:   duration,
	}
}

// Helper function to convert job.Status to pb.JobStatus
func (s *JobServiceServer) statusToProto(status job.Status) pb.JobStatus {
	switch status {
	case job.StatusPending:
		return pb.JobStatus_PENDING
	case job.StatusRunning:
		return pb.JobStatus_RUNNING
	case job.StatusCompleted:
		return pb.JobStatus_COMPLETED
	case job.StatusFailed:
		return pb.JobStatus_FAILED
	case job.StatusCancelled:
		return pb.JobStatus_CANCELLED
	default:
		return pb.JobStatus_PENDING
	}
}
