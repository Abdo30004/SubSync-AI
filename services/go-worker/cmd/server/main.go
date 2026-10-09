package main

import (
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	
	grpcclient "github.com/subtitle-sync/go-worker/internal/grpc"
	"github.com/subtitle-sync/go-worker/internal/pb/syncpb"
	"github.com/subtitle-sync/go-worker/internal/pipeline"
)

func main() {
	log.Println("Starting Go Worker Service...")

	pythonUrl := os.Getenv("PYTHON_TRANSLATOR_URL")
	if pythonUrl == "" {
		pythonUrl = "localhost:50052"
	}

	// Initialize Translation Client
	tClient, err := grpcclient.NewTranslationClient(pythonUrl)
	if err != nil {
		log.Fatalf("Failed to connect to Python Translator: %v", err)
	}

	// Initialize Coordinator
	coord := &pipeline.Coordinator{
		TranslationClient: tClient,
	}

	// Setup gRPC Server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	
	grpcServer := grpc.NewServer()
	syncServer := &grpcclient.SyncServer{
		Coordinator: coord,
	}
	
	syncpb.RegisterSyncServiceServer(grpcServer, syncServer)
	
	log.Println("Go Worker listening on port 50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
