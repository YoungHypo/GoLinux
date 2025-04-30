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
	jm := manager.NewJobManager(1000)

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
	fmt.Println("  help                  show help information")
	fmt.Println("\nExamples:")
	fmt.Println("  ./golinux run ls -la")
	fmt.Println("  ./golinux run echo \"Hello, World!\"")
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
			} else {
				fmt.Printf("%s", j.ErrorMsg)
			}
			break
		}

		fmt.Printf(".")
		time.Sleep(500 * time.Millisecond)
	}
}
