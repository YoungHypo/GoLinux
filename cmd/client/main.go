package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "GoLinux/proto"
)

func main() {
	var (
		serverAddr = flag.String("server", "localhost:50051", "gRPC server address")
		timeout    = flag.Int("timeout", 60, "Command timeout in seconds")
	)
	flag.Parse()

	if len(flag.Args()) == 0 {
		printUsage()
		os.Exit(1)
	}

	// Connect to gRPC server
	conn, err := grpc.NewClient(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	client := pb.NewJobServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	command := flag.Args()[0]
	args := flag.Args()[1:]

	switch command {
	case "run":
		runCommand(ctx, client, args, *timeout)
	case "status":
		if len(args) == 0 {
			log.Fatal("Job ID is required for status command")
		}
		getJobStatus(ctx, client, args[0])
	case "stop":
		if len(args) == 0 {
			log.Fatal("Job ID is required for stop command")
		}
		stopJob(ctx, client, args[0])
	case "list":
		listJobs(ctx, client)
	case "clean":
		cleanJobs(ctx, client)
	case "stream":
		if len(args) == 0 {
			log.Fatal("Job ID is required for stream command")
		}
		streamJobStatus(ctx, client, args[0])
	default:
		log.Fatalf("Unknown command: %s", command)
	}
}

func printUsage() {
	fmt.Println("GoLinux gRPC Client")
	fmt.Println("Usage:")
	fmt.Println("  golinux-client [options] <command> [args...]")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -server string    gRPC server address (default: localhost:50051)")
	fmt.Println("  -timeout int      Command timeout in seconds (default: 60)")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  run <command>     Execute a Linux command")
	fmt.Println("  status <job-id>   Get job status")
	fmt.Println("  stop <job-id>     Stop a running job")
	fmt.Println("  list              List all jobs")
	fmt.Println("  clean             Delete all job files")
	fmt.Println("  stream <job-id>   Stream job status updates")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  golinux-client run ls -la")
	fmt.Println("  golinux-client run sleep 30")
	fmt.Println("  golinux-client status abc123")
	fmt.Println("  golinux-client stop abc123")
	fmt.Println("  golinux-client list")
	fmt.Println("  golinux-client clean")
	fmt.Println("  golinux-client stream abc123")
}

func runCommand(ctx context.Context, client pb.JobServiceClient, args []string, timeout int) {
	if len(args) == 0 {
		log.Fatal("Command is required")
	}

	command := strings.Join(args, " ")

	// Create job
	resp, err := client.CreateJob(ctx, &pb.CreateJobRequest{
		Command: command,
		Timeout: int32(timeout),
	})
	if err != nil {
		log.Fatalf("Failed to create job: %v", err)
	}

	if !resp.Success {
		log.Fatalf("Failed to create job: %s", resp.Error)
	}

	fmt.Printf("Job created: %s\n", resp.JobId)

	// Poll for completion
	for {
		statusResp, err := client.GetJobStatus(ctx, &pb.GetJobStatusRequest{
			JobId: resp.JobId,
		})
		if err != nil {
			log.Fatalf("Failed to get job status: %v", err)
		}

		if statusResp.IsComplete {
			switch statusResp.Status {
			case pb.JobStatus_COMPLETED:
				fmt.Printf("%s", statusResp.Result)
			case pb.JobStatus_CANCELLED:
				fmt.Printf("Job was cancelled by user\n")
			case pb.JobStatus_FAILED:
				fmt.Printf("Error message: %s\n", statusResp.ErrorMessage)
			}
			break
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func getJobStatus(ctx context.Context, client pb.JobServiceClient, jobID string) {
	resp, err := client.GetJobStatus(ctx, &pb.GetJobStatusRequest{
		JobId: jobID,
	})
	if err != nil {
		log.Fatalf("Failed to get job status: %v", err)
	}

	fmt.Printf("Job ID      : %s\n", resp.JobId)
	fmt.Printf("Command     : %s\n", resp.Command)
	fmt.Printf("Status      : %s\n", resp.Status.String())
	fmt.Printf("Created At  : %s\n", formatTime(resp.CreatedAt))
	fmt.Printf("Started At  : %s\n", formatTime(resp.StartedAt))
	fmt.Printf("Completed At: %s\n", formatTime(resp.CompletedAt))
	fmt.Printf("Exit Code   : %d\n", resp.ExitCode)
	fmt.Printf("PID         : %d\n", resp.Pid)

	if resp.DurationMs > 0 {
		fmt.Printf("Duration    : %dms\n", resp.DurationMs)
	}

	if resp.Result != "" {
		fmt.Printf("Result:\n%s\n", resp.Result)
	}
	if resp.ErrorMessage != "" {
		if resp.Status == pb.JobStatus_CANCELLED {
			fmt.Printf("Notice      : Job was cancelled by user\n")
		} else {
			fmt.Printf("Error       : %s\n", resp.ErrorMessage)
		}
	}
}

func stopJob(ctx context.Context, client pb.JobServiceClient, jobID string) {
	resp, err := client.StopJob(ctx, &pb.StopJobRequest{
		JobId: jobID,
	})
	if err != nil {
		log.Fatalf("Failed to stop job: %v", err)
	}

	if resp.Success {
		fmt.Printf("Job %s stopped successfully\n", jobID)
	} else {
		fmt.Printf("Failed to stop job: %s\n", resp.Error)
	}
}

func listJobs(ctx context.Context, client pb.JobServiceClient) {
	resp, err := client.ListJobs(ctx, &pb.ListJobsRequest{})
	if err != nil {
		log.Fatalf("Failed to list jobs: %v", err)
	}

	if len(resp.Jobs) == 0 {
		fmt.Println("No jobs found")
		return
	}

	fmt.Printf("%-36s %-10s %-20s %-8s %-s\n", "ID", "STATUS", "CREATED", "PID", "COMMAND")

	for _, job := range resp.Jobs {
		id := job.Id

		command := job.Command
		if len(command) > 40 {
			command = command[:37] + "..."
		}

		pid := "-"
		if job.Pid > 0 {
			pid = strconv.Itoa(int(job.Pid))
		}

		fmt.Printf("%-36s %-10s %-20s %-8s %-s\n",
			id,
			job.Status.String(),
			formatTime(job.CreatedAt),
			pid,
			command,
		)
	}

	fmt.Printf("\nTotal: %d jobs\n", resp.TotalCount)
}

func cleanJobs(ctx context.Context, client pb.JobServiceClient) {
	resp, err := client.CleanJobs(ctx, &pb.CleanJobsRequest{
		Confirm: true,
	})
	if err != nil {
		log.Fatalf("Failed to clean jobs: %v", err)
	}

	if resp.Success {
		fmt.Printf("Successfully deleted %d job files\n", resp.DeletedCount)
	} else {
		fmt.Printf("Failed to clean jobs: %s\n", resp.Error)
	}
}

func streamJobStatus(ctx context.Context, client pb.JobServiceClient, jobID string) {
	stream, err := client.StreamJobStatus(ctx, &pb.GetJobStatusRequest{
		JobId: jobID,
	})
	if err != nil {
		log.Fatalf("Failed to start status stream: %v", err)
	}

	fmt.Printf("Streaming status for job %s...\n", jobID)
	fmt.Println(strings.Repeat("-", 50))

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Failed to receive status: %v", err)
		}

		fmt.Printf("[%s] Status: %s",
			time.Now().Format("15:04:05"),
			resp.Status.String())

		if resp.Pid > 0 {
			fmt.Printf(" (PID: %d)", resp.Pid)
		}

		if resp.DurationMs > 0 {
			fmt.Printf(" Duration: %dms", resp.DurationMs)
		}

		fmt.Println()

		if resp.IsComplete {
			fmt.Println("Job completed")
			if resp.Result != "" {
				fmt.Printf("Result:\n%s\n", resp.Result)
			}
			if resp.ErrorMessage != "" {
				if resp.Status == pb.JobStatus_CANCELLED {
					fmt.Printf("Notice: Job was cancelled by user\n")
				} else {
					fmt.Printf("Error: %s\n", resp.ErrorMessage)
				}
			}
			break
		}
	}
}

func formatTime(timeStr string) string {
	if timeStr == "" || timeStr == "0001-01-01T00:00:00Z" {
		return "-"
	}

	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return timeStr
	}

	return t.Format("2025-01-01 00:00:00")
}
