package search

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

type Track struct {
	ID     string
	Title  string
	Artist string
	URL    string
}

type YTSearcher struct {
	ytDlpPath    string
	resultsLimit int
}

func NewYTSearcher(ytDlpPath string, resultsLimit int) *YTSearcher {
	return &YTSearcher{
		ytDlpPath:    ytDlpPath,
		resultsLimit: resultsLimit,
	}
}

func (s *YTSearcher) Search(query string) ([]Track, error) {
	cmd := exec.Command(
		s.ytDlpPath,
		fmt.Sprintf("ytsearch%d:%s", s.resultsLimit, query),
		"--dump-json",
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run ytsearch command: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var tracks []Track

	for _, line := range lines {
		if line == "" {
			continue
		}
		var parsed struct {
			ID         string `json:"id"`
			Title      string `json:"title"`
			Channel    string `json:"channel"`
			WebpageURL string `json:"webpage_url"`
		}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			log.Printf("search parse error: %v", err)
			continue
		}
		tr := Track{
			ID:     parsed.ID,
			Title:  parsed.Title,
			Artist: parsed.Channel,
			URL:    parsed.WebpageURL,
		}
		tracks = append(tracks, tr)
	}

	return tracks, nil
}

func (s *YTSearcher) DownloadAudio(track Track, outPath string) error {
	cmd := exec.Command(
		s.ytDlpPath,
		"-x",
		"--audio-format", "mp3",
		"--output", outPath,
		track.URL,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download audio: %w", err)
	}
	return nil
}
