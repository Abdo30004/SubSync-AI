package srt

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type SubtitleBlock struct {
	Index int
	Start time.Duration
	End   time.Duration
	Text  string
}

// Parse reads an SRT file and returns an array of SubtitleBlocks
func Parse(path string) ([]SubtitleBlock, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var blocks []SubtitleBlock
	var currentBlock SubtitleBlock
	var textLines []string
	state := 0 // 0=index, 1=time, 2=text

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			if state == 2 {
				currentBlock.Text = strings.Join(textLines, "\n")
				blocks = append(blocks, currentBlock)
				textLines = nil
				state = 0
			}
			continue
		}

		switch state {
		case 0:
			idx, err := strconv.Atoi(line)
			if err == nil {
				currentBlock.Index = idx
				state = 1
			}
		case 1:
			times := strings.Split(line, " --> ")
			if len(times) == 2 {
				currentBlock.Start, _ = parseTime(times[0])
				currentBlock.End, _ = parseTime(times[1])
				state = 2
			}
		case 2:
			textLines = append(textLines, line)
		}
	}
	// Append last block if file doesn't end with newline
	if state == 2 {
		currentBlock.Text = strings.Join(textLines, "\n")
		blocks = append(blocks, currentBlock)
	}

	return blocks, scanner.Err()
}

func parseTime(tStr string) (time.Duration, error) {
	tStr = strings.ReplaceAll(tStr, ",", ".")
	// Format is HH:MM:SS.mmm
	parts := strings.Split(tStr, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid time format")
	}
	
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	sParts := strings.Split(parts[2], ".")
	s, _ := strconv.Atoi(sParts[0])
	ms, _ := strconv.Atoi(sParts[1])

	dur := time.Duration(h)*time.Hour +
		time.Duration(m)*time.Minute +
		time.Duration(s)*time.Second +
		time.Duration(ms)*time.Millisecond
	return dur, nil
}

// FormatTime formats a duration back into SRT time format
func FormatTime(d time.Duration) string {
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	d -= s * time.Second
	ms := d / time.Millisecond

	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
