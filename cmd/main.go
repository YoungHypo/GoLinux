package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"GoLinux/pkg/job"
	"GoLinux/pkg/manager"
)

// Simple command line tool for executing Linux commands
func main() {
	jm, err := manager.NewJobManager(1000, "jobs.db")
	if err != nil {
		log.Fatalf("Failed to initialize job manager: %v", err)
	}
	defer jm.Close()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "run":
		if len(os.Args) < 3 {
			fmt.Println("Error: missing command parameters")
			printUsage()
			os.Exit(1)
		}
		runCommand(jm, os.Args[2:])
	case "status":
		if len(os.Args) < 3 {
			fmt.Println("Error: missing job ID")
			printUsage()
			os.Exit(1)
		}
		getJobStatus(jm, os.Args[2])
	case "list":
		listJobs(jm)
	case "clean":
		cleanJobs(jm)
	case "stop":
		if len(os.Args) < 3 {
			fmt.Println("Error: missing job ID")
			printUsage()
			os.Exit(1)
		}
		stopJob(jm, os.Args[2])
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Linux command execution tool usage:")
	fmt.Println("  run <command>         execute command")
	fmt.Println("  status <job-id>       query job status")
	fmt.Println("  list                  list all jobs")
	fmt.Println("  clean                 delete all job history")
	fmt.Println("  stop <job-id>         stop a running job")
	fmt.Println("  help                  show help information")
	fmt.Println("\nExamples:")
	fmt.Println("  ./golinux run ls -la")
	fmt.Println("  ./golinux status <job-id>")
	fmt.Println("  ./golinux stop <job-id>")
}

func runCommand(jm *manager.JobManager, args []string) {
	command := strings.Join(args, " ")

	j, err := jm.CreateJob(command, 60) // 60 seconds timeout
	if err != nil {
		log.Fatalf("Create job failed: %v", err)
	}

	for {
		j, err = jm.GetJob(j.ID)
		if err != nil {
			log.Fatalf("Get job status failed: %v", err)
		}

		if j.IsComplete() {
			if j.Status == job.StatusCompleted {
				fmt.Printf("%s", j.Result)
			} else if j.Status == job.StatusCancelled {
				fmt.Printf("Job was cancelled by user\n")
			} else {
				fmt.Printf("Error message: %s\n", j.ErrorMsg)
			}
			break
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func getJobStatus(jm *manager.JobManager, jobID string) {
	j, err := jm.GetJob(jobID)
	if err != nil {
		log.Fatalf("Get job failed: %v", err)
	}

	fmt.Printf("Job ID      : %s\n", j.ID)
	fmt.Printf("Command     : %s\n", j.Command)
	fmt.Printf("Status      : %s\n", j.Status)
	fmt.Printf("Created at  : %s\n", j.CreatedAt.Format("2006-01-02 15:04:05"))

	if j.Status == job.StatusRunning {
		fmt.Printf("Running for : %s\n", j.Duration())
	} else if j.IsComplete() {
		fmt.Printf("Completed at: %s\n", j.CompletedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Duration    : %s\n", j.Duration())
	}

	if j.Status == job.StatusCompleted {
		fmt.Printf("Output      : %s\n", j.Result)
	} else if j.Status == job.StatusFailed {
		fmt.Printf("Error       : %s\n", j.ErrorMsg)
		fmt.Printf("Exit code   : %d\n", j.ExitCode)
	} else if j.Status == job.StatusCancelled {
		fmt.Printf("Notice      : Job was cancelled by user\n")
		fmt.Printf("Process ID  : %d (terminated)\n", j.Pid)
	}
}

func listJobs(jm *manager.JobManager) {
	jobs, err := jm.ListJobs("")
	if err != nil {
		log.Fatalf("Failed to list jobs: %v", err)
	}

	fmt.Printf("Total %d jobs:\n", len(jobs))

	if len(jobs) == 0 {
		fmt.Println("No job records")
		return
	}

	fmt.Println("ID                                   Command                Status     Created At")
	fmt.Println("--------------------------------------------------------------------------------")

	for _, j := range jobs {
		cmdShort := j.Command
		if len(cmdShort) > 20 {
			cmdShort = cmdShort[:17] + "..."
		}

		fmt.Printf("%-36s %-20s %-10s %s\n",
			j.ID,
			cmdShort,
			j.Status,
			j.CreatedAt.Format("2006-01-02 15:04:05"))
	}
}

func cleanJobs(jm *manager.JobManager) {
	fmt.Println("Cleaning all job history...")

	count, err := jm.CleanJobs()
	if err != nil {
		log.Fatalf("Failed to clean jobs: %v", err)
	}

	fmt.Printf("Successfully deleted %d job files\n", count)
	fmt.Println("All job history has been cleared")
}

func stopJob(jm *manager.JobManager, jobID string) {
	err := jm.StopJob(jobID)
	if err != nil {
		log.Fatalf("Failed to stop job: %v", err)
	}

	fmt.Printf("Job %s stopped successfully\n", jobID)
	// Display the current status after stopping
	getJobStatus(jm, jobID)
}
