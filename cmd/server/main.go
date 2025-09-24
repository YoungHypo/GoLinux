package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	grpcserver "GoLinux/pkg/grpc"
	"GoLinux/pkg/manager"
	pb "GoLinux/proto"
)

func main() {
	var (
		port    = flag.Int("port", 50051, "gRPC server port")
		maxJobs = flag.Int("max-jobs", 1000, "Maximum number of jobs")
		dbPath  = flag.String("db-path", "jobs.db", "SQLite database path")
	)
	flag.Parse()

	jm, err := manager.NewJobManager(*maxJobs, *dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize job manager: %v", err)
	}
	defer jm.Close()

	log.Printf("Job manager created with max jobs: %d", *maxJobs)
	log.Printf("Database path: %s", *dbPath)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", *port, err)
	}

	s := grpc.NewServer()

	jobService := grpcserver.NewJobServiceServer(jm)
	pb.RegisterJobServiceServer(s, jobService)

	reflection.Register(s)

	log.Printf("GoLinux gRPC server starting on port %d", *port)
	log.Printf("Job storage: SQLite database at %s", *dbPath)
	log.Println("Server is ready to accept connections...")

	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}
