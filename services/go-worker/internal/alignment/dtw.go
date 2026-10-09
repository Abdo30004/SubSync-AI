package alignment

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/subtitle-sync/go-worker/internal/srt"
)

// normalizeText lowercases, strips punctuation, and collapses whitespace
// so that Levenshtein comparison isn't thrown off by casing/punctuation differences.
func normalizeText(s string) string {
	s = strings.ToLower(s)
	// Replace newlines with spaces (multi-line subtitle blocks)
	s = strings.ReplaceAll(s, "\n", " ")
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) {
			sb.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	aRunes, bRunes := []rune(a), []rune(b)
	m, n := len(aRunes), len(bRunes)
	if m == 0 {
		return n
	}
	if n == 0 {
		return m
	}

	// Use two-row optimization to save memory
	prev := make([]int, n+1)
	curr := make([]int, n+1)
	for j := 0; j <= n; j++ {
		prev[j] = j
	}

	for i := 1; i <= m; i++ {
		curr[0] = i
		for j := 1; j <= n; j++ {
			cost := 1
			if aRunes[i-1] == bRunes[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			min := del
			if ins < min {
				min = ins
			}
			if sub < min {
				min = sub
			}
			curr[j] = min
		}
		prev, curr = curr, prev
	}
	return prev[n]
}

// normalizedDistance returns a 0.0–1.0 distance between two strings.
func normalizedDistance(a, b string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	dist := levenshtein(a, b)
	maxLen := len([]rune(a))
	if len([]rune(b)) > maxLen {
		maxLen = len([]rune(b))
	}
	if maxLen == 0 {
		return 0
	}
	return float64(dist) / float64(maxLen)
}

// WhisperSegment holds a transcribed segment with timing.
type WhisperSegment struct {
	Text  string
	Start time.Duration
	End   time.Duration
}

// AlignDTW performs full Dynamic Time Warping between SRT blocks (translated text)
// and Whisper segments to find the globally optimal alignment path.
// It returns the original SRT blocks with timestamps corrected to match actual speech.
//
// Parameters:
//   - whisperSegs: Whisper transcription segments with text and timing
//   - srtBlocks: SRT blocks whose .Text has already been translated to match Whisper language
//   - originalBlocks: the original-language SRT blocks (same indices) to restore text from
func AlignDTW(whisperSegs []WhisperSegment, srtBlocks []srt.SubtitleBlock, originalBlocks []srt.SubtitleBlock) []srt.SubtitleBlock {
	N := len(srtBlocks)    // rows = SRT blocks
	M := len(whisperSegs)  // cols = Whisper segments

	log.Printf("Running full DTW alignment: %d SRT blocks × %d Whisper segments", N, M)

	result := make([]srt.SubtitleBlock, N)

	if M == 0 || N == 0 {
		log.Printf("No whisper segments or SRT blocks — returning original timestamps")
		copy(result, originalBlocks)
		return result
	}

	// Step 1: Build normalized cost matrix
	// cost[i][j] = normalized Levenshtein distance between srt[i] and whisper[j]
	cost := make([][]float64, N)
	for i := 0; i < N; i++ {
		cost[i] = make([]float64, M)
		normSrt := normalizeText(srtBlocks[i].Text)
		for j := 0; j < M; j++ {
			normW := normalizeText(whisperSegs[j].Text)
			cost[i][j] = normalizedDistance(normSrt, normW)
		}
	}

	// Step 2: Build cumulative DTW matrix with Sakoe-Chiba band
	// The band limits how far off-diagonal we allow the path to go.
	// This both improves speed and prevents pathological warps.
	bandWidth := int(math.Max(float64(N), float64(M))/2) + 10
	inf := math.Inf(1)

	dtw := make([][]float64, N)
	// backtrack direction: 0=diagonal, 1=left (skip whisper), 2=up (skip srt)
	back := make([][]int, N)
	for i := 0; i < N; i++ {
		dtw[i] = make([]float64, M)
		back[i] = make([]int, M)
		for j := 0; j < M; j++ {
			dtw[i][j] = inf
		}
	}

	dtw[0][0] = cost[0][0]
	// Initialize first row (skip whisper segments to reach j)
	for j := 1; j < M && j <= bandWidth; j++ {
		dtw[0][j] = dtw[0][j-1] + cost[0][j]
		back[0][j] = 1 // came from left
	}
	// Initialize first column (skip SRT blocks to reach i)
	for i := 1; i < N && i <= bandWidth; i++ {
		dtw[i][0] = dtw[i-1][0] + cost[i][0]
		back[i][0] = 2 // came from above
	}

	// Fill the DTW matrix within the Sakoe-Chiba band
	for i := 1; i < N; i++ {
		// Compute expected j position on the diagonal
		expectedJ := int(float64(i) * float64(M) / float64(N))
		jMin := expectedJ - bandWidth
		if jMin < 1 {
			jMin = 1
		}
		jMax := expectedJ + bandWidth
		if jMax >= M {
			jMax = M - 1
		}

		for j := jMin; j <= jMax; j++ {
			c := cost[i][j]

			// Diagonal: both advance
			diag := inf
			if dtw[i-1][j-1] < inf {
				diag = dtw[i-1][j-1] + c
			}
			// Left: whisper advances, SRT stays (merge whisper segments)
			left := inf
			if dtw[i][j-1] < inf {
				left = dtw[i][j-1] + c
			}
			// Up: SRT advances, whisper stays (multiple SRT blocks map to same whisper)
			up := inf
			if dtw[i-1][j] < inf {
				up = dtw[i-1][j] + c
			}

			if diag <= left && diag <= up {
				dtw[i][j] = diag
				back[i][j] = 0
			} else if left <= up {
				dtw[i][j] = left
				back[i][j] = 1
			} else {
				dtw[i][j] = up
				back[i][j] = 2
			}
		}
	}

	// Step 3: Traceback — find the optimal warping path
	// srtToWhisper[i] = the whisper segment index that SRT block i maps to
	srtToWhisper := make([]int, N)
	for i := range srtToWhisper {
		srtToWhisper[i] = -1
	}

	i, j := N-1, M-1

	// If dtw[N-1][M-1] is inf, find the closest reachable end point
	if math.IsInf(dtw[i][j], 1) {
		bestVal := inf
		for jj := 0; jj < M; jj++ {
			if dtw[N-1][jj] < bestVal {
				bestVal = dtw[N-1][jj]
				j = jj
			}
		}
		if math.IsInf(bestVal, 1) {
			// DTW completely failed — fall back to linear mapping
			log.Printf("DTW matrix unreachable — falling back to linear timestamp mapping")
			return linearFallback(whisperSegs, originalBlocks)
		}
	}

	// Walk back through the matrix
	for i >= 0 && j >= 0 {
		srtToWhisper[i] = j
		if i == 0 && j == 0 {
			break
		}
		switch back[i][j] {
		case 0: // diagonal
			i--
			j--
		case 1: // came from left
			j--
		case 2: // came from above
			i--
		}
	}

	// Step 4: Apply Whisper timestamps to mapped SRT blocks
	anchorCount := 0
	for i := 0; i < N; i++ {
		result[i] = originalBlocks[i] // start with original text
		wIdx := srtToWhisper[i]
		if wIdx >= 0 && wIdx < M {
			origDuration := originalBlocks[i].End - originalBlocks[i].Start
			result[i].Start = whisperSegs[wIdx].Start
			result[i].End = whisperSegs[wIdx].End

			// If the whisper segment end is before or equal to start (degenerate),
			// preserve the original block duration
			if result[i].End <= result[i].Start {
				result[i].End = result[i].Start + origDuration
			}
			anchorCount++
		}
	}

	log.Printf("DTW alignment complete: %d/%d blocks mapped to whisper segments", anchorCount, N)

	// Step 5: Sanity — ensure timestamps are monotonically increasing
	for i := 1; i < N; i++ {
		if result[i].Start < result[i-1].Start {
			result[i].Start = result[i-1].End
			if result[i].End <= result[i].Start {
				origDur := originalBlocks[i].End - originalBlocks[i].Start
				result[i].End = result[i].Start + origDur
			}
		}
	}

	// Clamp negative timestamps to zero
	for i := range result {
		if result[i].Start < 0 {
			result[i].Start = 0
		}
		if result[i].End < 0 {
			result[i].End = 0
		}
	}

	return result
}

// linearFallback maps SRT blocks to Whisper segments proportionally
// when DTW completely fails (e.g., text is too different).
func linearFallback(whisperSegs []WhisperSegment, originalBlocks []srt.SubtitleBlock) []srt.SubtitleBlock {
	N := len(originalBlocks)
	M := len(whisperSegs)
	result := make([]srt.SubtitleBlock, N)

	if M == 0 {
		copy(result, originalBlocks)
		return result
	}

	// Map SRT blocks proportionally across the whisper timeline
	whisperStart := whisperSegs[0].Start
	whisperEnd := whisperSegs[M-1].End
	srtStart := originalBlocks[0].Start
	srtEnd := originalBlocks[N-1].End

	totalSrtDur := srtEnd - srtStart
	totalWhisperDur := whisperEnd - whisperStart

	if totalSrtDur <= 0 {
		totalSrtDur = 1 // prevent division by zero
	}

	scale := float64(totalWhisperDur) / float64(totalSrtDur)

	log.Printf("Linear fallback: scaling by %.3f (whisper range: %.1fs, srt range: %.1fs)",
		scale, totalWhisperDur.Seconds(), totalSrtDur.Seconds())

	for i := 0; i < N; i++ {
		result[i] = originalBlocks[i]
		offsetFromStart := originalBlocks[i].Start - srtStart
		dur := originalBlocks[i].End - originalBlocks[i].Start

		newStart := whisperStart + time.Duration(float64(offsetFromStart)*scale)
		newEnd := newStart + time.Duration(float64(dur)*scale)

		result[i].Start = newStart
		result[i].End = newEnd
	}
	return result
}

// FormatBlocksToString serializes subtitle blocks back to SRT text format.
func FormatBlocksToString(blocks []srt.SubtitleBlock) string {
	var out string
	for _, b := range blocks {
		out += fmt.Sprintf("%d\n%s --> %s\n%s\n\n",
			b.Index,
			srt.FormatTime(b.Start),
			srt.FormatTime(b.End),
			b.Text,
		)
	}
	return out
}
