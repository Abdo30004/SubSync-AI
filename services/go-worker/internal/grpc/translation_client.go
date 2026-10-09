package grpcclient

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"github.com/subtitle-sync/go-worker/internal/pb/translationpb"
	"github.com/subtitle-sync/go-worker/internal/pipeline"
)

type TranslationClient struct {
	client translationpb.TranslationServiceClient
}

func NewTranslationClient(addr string) (*TranslationClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &TranslationClient{
		client: translationpb.NewTranslationServiceClient(conn),
	}, nil
}

func (c *TranslationClient) Translate(blocks []string, source, target string) ([]string, error) {
	resp, err := c.client.TranslateBlocks(context.Background(), &translationpb.TranslationRequest{
		TextBlocks:     blocks,
		SourceLanguage: source,
		TargetLanguage: target,
	})
	if err != nil {
		return nil, err
	}
	return resp.TranslatedBlocks, nil
}

func (c *TranslationClient) Transcribe(audioPath string) (*pipeline.TranscribeResult, error) {
	resp, err := c.client.TranscribeAudio(context.Background(), &translationpb.TranscribeRequest{
		AudioPath: audioPath,
	})
	if err != nil {
		return nil, err
	}
	return &pipeline.TranscribeResult{
		Segments:         resp.Segments,
		DetectedLanguage: resp.DetectedLanguage,
	}, nil
}
