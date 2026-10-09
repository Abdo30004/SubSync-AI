package pipeline

import (
	"log"
	"path/filepath"
	"time"

	"github.com/subtitle-sync/go-worker/internal/alignment"
	"github.com/subtitle-sync/go-worker/internal/audio"
	"github.com/subtitle-sync/go-worker/internal/pb/syncpb"
	"github.com/subtitle-sync/go-worker/internal/pb/translationpb"
	"github.com/subtitle-sync/go-worker/internal/srt"
)

// TranscribeResult holds the output of a Whisper transcription call.
// Defined here (not in grpc package) to avoid an import cycle.
type TranscribeResult struct {
	Segments         []*translationpb.WhisperSegment
	DetectedLanguage string
}

type Translator interface {
	Translate(blocks []string, source, target string) ([]string, error)
	Transcribe(audioPath string) (*TranscribeResult, error)
}

type Coordinator struct {
	TranslationClient Translator
}

func (c *Coordinator) ProcessJob(req *syncpb.SyncRequest, stream syncpb.SyncService_SyncSubtitlesServer) error {
	log.Printf("=== Starting job %s ===", req.JobId)
	log.Printf("  Video: %s", req.MovieFilePath)
	log.Printf("  SRT:   %s", req.OriginalSrtPath)
	log.Printf("  SRT Language: %s", req.Language)

	baseDir := "/app/shared_data"
	videoPath := filepath.Join(baseDir, req.MovieFilePath)
	srtPath := filepath.Join(baseDir, req.OriginalSrtPath)

	// ──────────────────────────────────────────────
	// STEP 1: Extract audio from video (16kHz mono WAV)
	// ──────────────────────────────────────────────
	log.Printf("[Step 1] Extracting audio from video...")
	audioPath, err := audio.ExtractAudio(videoPath, baseDir)
	if err != nil {
		log.Printf("Audio extraction failed: %v", err)
		return sendError(stream, "Audio extraction failed: "+err.Error())
	}
	log.Printf("[Step 1] Audio extracted: %s", audioPath)

	// ──────────────────────────────────────────────
	// STEP 2: Whisper speech-to-text (auto-detect language)
	// ──────────────────────────────────────────────
	log.Printf("[Step 2] Transcribing audio via Whisper...")
	transcribeResult, err := c.TranslationClient.Transcribe(audioPath)
	if err != nil {
		log.Printf("Whisper transcription failed: %v", err)
		return sendError(stream, "Whisper transcription failed: "+err.Error())
	}

	audioLang := transcribeResult.DetectedLanguage
	if audioLang == "" {
		audioLang = "en"
	}
	log.Printf("[Step 2] Whisper transcribed %d segments (detected audio language: %s)",
		len(transcribeResult.Segments), audioLang)

	// Convert proto segments to alignment segments
	var whisperSegs []alignment.WhisperSegment
	for _, seg := range transcribeResult.Segments {
		whisperSegs = append(whisperSegs, alignment.WhisperSegment{
			Text:  seg.Text,
			Start: time.Duration(seg.Start * float64(time.Second)),
			End:   time.Duration(seg.End * float64(time.Second)),
		})
	}

	// ──────────────────────────────────────────────
	// STEP 3: Parse the original SRT file
	// ──────────────────────────────────────────────
	log.Printf("[Step 3] Parsing SRT file...")
	blocks, err := srt.Parse(srtPath)
	if err != nil {
		log.Printf("SRT parse failed: %v", err)
		return sendError(stream, "SRT parse failed: "+err.Error())
	}
	log.Printf("[Step 3] Parsed %d subtitle blocks", len(blocks))

	// ──────────────────────────────────────────────
	// STEP 4: Translate SRT text to match audio language
	// ──────────────────────────────────────────────
	// We need the SRT text in the SAME language as the audio
	// so the DTW text matching works.
	srtLang := req.Language
	if srtLang == "" {
		srtLang = "en"
	}

	// Build translated blocks for alignment (same structure, translated text)
	translatedBlocks := make([]srt.SubtitleBlock, len(blocks))
	copy(translatedBlocks, blocks)

	if srtLang != audioLang {
		log.Printf("[Step 4] Translating %d SRT blocks from '%s' to '%s' for alignment...",
			len(blocks), srtLang, audioLang)

		var originalTexts []string
		for _, b := range blocks {
			originalTexts = append(originalTexts, b.Text)
		}

		translatedTexts, err := c.TranslationClient.Translate(originalTexts, srtLang, audioLang)
		if err != nil {
			log.Printf("Translation failed (will try alignment with original text): %v", err)
			// Fall through with original text — DTW will do its best
		} else {
			for i := range translatedBlocks {
				translatedBlocks[i].Text = translatedTexts[i]
			}
			log.Printf("[Step 4] Translation complete")
		}
	} else {
		log.Printf("[Step 4] SRT language matches audio language (%s) — no translation needed", srtLang)
	}

	// ──────────────────────────────────────────────
	// STEP 5: Run full DTW alignment
	// ──────────────────────────────────────────────
	log.Printf("[Step 5] Running DTW alignment...")
	alignedBlocks := alignment.AlignDTW(whisperSegs, translatedBlocks, blocks)
	log.Printf("[Step 5] Alignment complete for %d blocks", len(alignedBlocks))

	// ──────────────────────────────────────────────
	// STEP 6: Stream results back in chunks
	// ──────────────────────────────────────────────
	chunkSize := 50 // larger chunks for efficiency
	totalChunks := (len(alignedBlocks) + chunkSize - 1) / chunkSize

	for i := 0; i < totalChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(alignedBlocks) {
			end = len(alignedBlocks)
		}

		srtContent := alignment.FormatBlocksToString(alignedBlocks[start:end])
		isFinal := (i == totalChunks-1)

		err := stream.Send(&syncpb.SyncChunk{
			Success:    true,
			ChunkIndex: int32(i),
			IsFinal:    isFinal,
			SrtContent: srtContent,
		})
		if err != nil {
			return err
		}
	}

	log.Printf("=== Job %s completed successfully ===", req.JobId)
	return nil
}

func sendError(stream syncpb.SyncService_SyncSubtitlesServer, msg string) error {
	return stream.Send(&syncpb.SyncChunk{
		Success:      false,
		IsFinal:      true,
		ErrorMessage: msg,
	})
}
