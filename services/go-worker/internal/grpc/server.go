package grpcclient // renamed package to avoid conflict with actual grpc package in other files, though it's internal/grpc

import (
	"log"

	"github.com/subtitle-sync/go-worker/internal/pb/syncpb"
	"github.com/subtitle-sync/go-worker/internal/pipeline"
)

type SyncServer struct {
	syncpb.UnimplementedSyncServiceServer
	Coordinator *pipeline.Coordinator
}

func (s *SyncServer) SyncSubtitles(req *syncpb.SyncRequest, stream syncpb.SyncService_SyncSubtitlesServer) error {
	log.Printf("Received SyncSubtitles request: %v", req)
	return s.Coordinator.ProcessJob(req, stream)
}
