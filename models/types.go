package models

import (
	"time"

	"github.com/google/uuid"
)

type Document struct {
	ID              string    `json:"id"`
	Path            string    `json:"path"`
	Type            string    `json:"type"`
	Content         string    `json:"content"`
	OriginalContent string    `json:"original_content,omitempty"`
	Indexed         time.Time `json:"indexed"`
}

type SearchResult struct {
	ID        string  `json:"id"`
	Path      string  `json:"path"`
	Type      string  `json:"type"`
	Content   string  `json:"content"`
	Score     float64 `json:"score"`
	IndexedAt string  `json:"indexed_at"`
}

func GenerateID() string {
	return uuid.New().String()
}

type SearchRequest struct {
    Query string `json:"query"`
}

type SearchResponse struct {
    Hits []struct {
        ID     string   `json:"_id"`
        Score  float64  `json:"_score"`
        Source Document `json:"_source"`
    } `json:"hits"`
}

type SimplifiedSearchResult struct {
    Total    int                  `json:"total"`
    Duration int                  `json:"duration_ms"`
    Results  []SimplifiedDocument `json:"results"`
}

type SimplifiedDocument struct {
    ID          string    `json:"id"`
    Path        string    `json:"path"`
    Type        string    `json:"type"`
    Indexed     time.Time `json:"indexed"`
    Score       float64   `json:"relevance_score"`
    Snippets    []string  `json:"snippets"`
    DownloadURL string    `json:"download_url"`
    ViewURL     string    `json:"view_url,omitempty"`
}