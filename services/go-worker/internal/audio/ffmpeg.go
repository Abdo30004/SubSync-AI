package audio

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

type SpeechSegment struct {
	Start time.Duration
	End   time.Duration
}

// ExtractAudio extracts a 16kHz mono wav file from the input video for AI processing
func ExtractAudio(videoPath string, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, "audio.wav")
	log.Printf("Extracting audio from %s to %s", videoPath, outputPath)
	
	// -y overwrites, -vn removes video, -ac 1 makes mono, -ar 16000 makes 16kHz
	cmd := exec.Command("ffmpeg", "-y", "-i", videoPath, "-vn", "-ac", "1", "-ar", "16000", outputPath)
	
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg extraction failed: %w, output: %s", err, string(output))
	}
	return outputPath, nil
}

// DetectSpeech uses ffmpeg silencedetect to find actual speech timings in the audio
func DetectSpeech(audioPath string) ([]SpeechSegment, error) {
	log.Printf("Detecting speech segments in %s using ffmpeg silencedetect...", audioPath)
	
	// Run silencedetect filter: silence noise threshold -30dB, minimum silence duration 0.3s
	cmd := exec.Command("ffmpeg", "-i", audioPath, "-af", "silencedetect=noise=-30dB:d=0.3", "-f", "null", "-")
	output, _ := cmd.CombinedOutput()
	
	outStr := string(output)
	
	// Parse silence_start and silence_end
	reStart := regexp.MustCompile(`silence_start: ([\d\.]+)`)
	reEnd := regexp.MustCompile(`silence_end: ([\d\.]+)`)
	
	starts := reStart.FindAllStringSubmatch(outStr, -1)
	ends := reEnd.FindAllStringSubmatch(outStr, -1)
	
	var segments []SpeechSegment
	var lastSpeechStart time.Duration = 0
	
	for i := 0; i < len(starts) && i < len(ends); i++ {
		silenceStartSec, _ := strconv.ParseFloat(starts[i][1], 64)
		silenceEndSec, _ := strconv.ParseFloat(ends[i][1], 64)
		
		silenceStart := time.Duration(silenceStartSec * float64(time.Second))
		silenceEnd := time.Duration(silenceEndSec * float64(time.Second))
		
		// Speech is the gap BEFORE this silence
		if silenceStart > lastSpeechStart {
			segments = append(segments, SpeechSegment{
				Start: lastSpeechStart,
				End:   silenceStart,
			})
		}
		lastSpeechStart = silenceEnd
	}
	
	// We don't have the total duration easily here, but usually the last speech segment extends to the end of the video.
	// For simplicity, we just add a 10s chunk at the end if there was a final silence.
	segments = append(segments, SpeechSegment{
		Start: lastSpeechStart,
		End:   lastSpeechStart + 10*time.Second,
	})
	
	log.Printf("Detected %d speech segments.", len(segments))
	return segments, nil
}
