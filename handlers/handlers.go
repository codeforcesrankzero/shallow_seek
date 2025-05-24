package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"encoding/base64"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/gin-gonic/gin"
	"github.com/shallowseek/batch"
	"github.com/shallowseek/cache"
	"github.com/shallowseek/config"
	"github.com/shallowseek/elasticsearch"
	"github.com/shallowseek/metrics"
	"github.com/shallowseek/models"
)

var (
	BatchProcessor = batch.NewBatchProcessor()
)

func init() {
	metrics.DocumentCount.Set(0)
}

func SearchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	log.Printf("[Search] Processing search request for query: %s", query)

	if cachedResults, err := cache.GetCachedSearchResult(query); err == nil && cachedResults != nil {
		log.Printf("[Search] Cache hit for query: %s", query)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		json.NewEncoder(w).Encode(cachedResults)
		return
	}

	log.Printf("[Search] Cache miss for query: %s", query)

	var buf bytes.Buffer
	searchQuery := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{
						"match_phrase": map[string]interface{}{
							"content": map[string]interface{}{
								"query": query,
								"slop":  0,
							},
						},
					},
					{
						"multi_match": map[string]interface{}{
							"query":  query,
							"fields": []string{"content"},
							"type":   "best_fields",
							"minimum_should_match": "75%",
						},
					},
				},
			},
		},
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				"content": map[string]interface{}{
					"fragment_size":       200,
					"number_of_fragments": 2,
					"pre_tags":           []string{"<mark>"},
					"post_tags":          []string{"</mark>"},
				},
			},
		},
		"_source": []string{"id", "path", "type", "content", "indexed"},
	}

	if err := json.NewEncoder(&buf).Encode(searchQuery); err != nil {
		log.Printf("[Search] Error encoding search query: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[Search] Executing search with query: %s", buf.String())

	startTime := time.Now()
	res, err := elasticsearch.Client.Search(
		elasticsearch.Client.Search.WithContext(context.Background()),
		elasticsearch.Client.Search.WithIndex("documents"),
		elasticsearch.Client.Search.WithBody(&buf),
		elasticsearch.Client.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		log.Printf("[Search] Error executing search: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Printf("[Search] Elasticsearch error: %s", res.String())
		http.Error(w, fmt.Sprintf("Error searching documents: %s", res.String()), http.StatusInternalServerError)
		return
	}

	var rawResult map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&rawResult); err != nil {
		log.Printf("[Search] Error decoding response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	simplifiedResult := models.SimplifiedSearchResult{
		Duration: int(time.Since(startTime).Milliseconds()),
		Results:  []models.SimplifiedDocument{},
	}

	if hits, ok := rawResult["hits"].(map[string]interface{}); ok {
		if total, ok := hits["total"].(map[string]interface{}); ok {
			if value, ok := total["value"].(float64); ok {
				simplifiedResult.Total = int(value)
			}
		}

		if hitsArray, ok := hits["hits"].([]interface{}); ok {
			log.Printf("[Search] Found %d hits", len(hitsArray))
			for _, hit := range hitsArray {
				hitMap, ok := hit.(map[string]interface{})
				if !ok {
					continue
				}

				id, _ := hitMap["_id"].(string)
				score, _ := hitMap["_score"].(float64)
				
				sourceMap, ok := hitMap["_source"].(map[string]interface{})
				if !ok {
					continue
				}

				path, _ := sourceMap["path"].(string)
				docType, _ := sourceMap["type"].(string)
				
				indexedStr, _ := sourceMap["indexed"].(string)
				indexed, _ := time.Parse(time.RFC3339, indexedStr)

				var snippets []string
				if highlightMap, ok := hitMap["highlight"].(map[string]interface{}); ok {
					if contentHighlights, ok := highlightMap["content"].([]interface{}); ok {
						for _, highlight := range contentHighlights {
							if snippet, ok := highlight.(string); ok {
								if !isBinaryContent(snippet) {
									snippets = append(snippets, snippet)
								}
							}
						}
					}
					if len(snippets) == 0 {
						if pathHighlights, ok := highlightMap["path"].([]interface{}); ok {
							for _, highlight := range pathHighlights {
								if snippet, ok := highlight.(string); ok {
									snippets = append(snippets, "Filename: "+snippet)
								}
							}
						}
					}
				}

				if len(snippets) == 0 {
					if strings.HasSuffix(strings.ToLower(docType), "pdf") {
						snippets = append(snippets, "PDF document contains matching content")
					} else {
						snippets = append(snippets, "Document contains matching content")
					}
				}

				downloadURL := fmt.Sprintf("/api/documents/%s/download", id)
				viewURL := fmt.Sprintf("/api/documents/%s/view", id)

				simplifiedResult.Results = append(simplifiedResult.Results, models.SimplifiedDocument{
					ID:          id,
					Path:        path,
					Type:        docType,
					Indexed:     indexed,
					Score:       score,
					Snippets:    snippets,
					DownloadURL: downloadURL,
					ViewURL:     viewURL,
				})
			}
		}
	}

	if err := cache.CacheSearchResult(query, simplifiedResult); err != nil {
		log.Printf("[Search] Failed to cache search results: %v", err)
	}

	log.Printf("[Search] Search completed in %dms with %d results", simplifiedResult.Duration, len(simplifiedResult.Results))

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	json.NewEncoder(w).Encode(simplifiedResult)
}

func isBinaryContent(text string) bool {
	if strings.HasPrefix(text, "%PDF-") {
		return true
	}

	binaryIndicators := []string{
		"\x00",
		"\x1B",
		"\x7F",
	}

	for _, indicator := range binaryIndicators {
		if strings.Contains(text, indicator) {
			return true
		}
	}

	nonPrintable := 0
	for _, r := range text {
		if !unicode.IsPrint(r) && !unicode.IsSpace(r) {
			nonPrintable++
		}
	}

	if float64(nonPrintable)/float64(len(text)) > 0.1 {
		return true
	}

	return false
}

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	health, err := elasticsearch.GetClusterHealth()
	if err != nil {
		log.Printf("[Status] Error getting cluster health: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	status := "OK"
	if health["status"] != "green" {
		status = "WARNING"
	}

	indexStats, err := elasticsearch.GetIndexStatus()
	if err != nil {
		log.Printf("[Status] Error getting index stats: %v", err)
	}

	docCount, err := elasticsearch.GetDocumentCount()
	if err != nil {
		log.Printf("[Status] Error getting document count: %v", err)
	}

	response := map[string]interface{}{
		"status":   status,
		"elastic":  health,
		"version":  "shallowseek-1.0",
		"uptime":   time.Since(config.StartTime).String(),
		"index":    indexStats,
		"documents": docCount,
	}

	log.Printf("[Status] Current status: %+v", response)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func UploadFileHandler(c *gin.Context) {
	log.Printf("[Upload] Starting file upload handler")
	
	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("[Upload] Error getting form file: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	log.Printf("[Upload] Received file: %s, size: %d bytes", file.Filename, file.Size)

	if file.Size > 10*1024*1024 {
		log.Printf("[Upload] File too large: %s (%d bytes)", file.Filename, file.Size)
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large (max 10MB)"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	supportedTypes := map[string]bool{
		".txt":  true,
		".pdf":  true,
		".doc":  true,
		".docx": true,
	}
	if !supportedTypes[ext] {
		log.Printf("[Upload] Unsupported file type: %s", ext)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported file type"})
		return
	}

	src, err := file.Open()
	if err != nil {
		log.Printf("[Upload] Error opening file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		log.Printf("[Upload] Error reading file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	log.Printf("[Upload] Successfully read file content, size: %d bytes", len(content))

	if len(content) == 0 {
		log.Printf("[Upload] Empty file content: %s", file.Filename)
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is empty"})
		return
	}

	var contentStr string
	if ext == ".pdf" {
		tmpFile, err := os.CreateTemp("", "upload-*.pdf")
		if err != nil {
			log.Printf("[Upload] Error creating temp file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process PDF"})
			return
		}
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()

		if _, err := tmpFile.Write(content); err != nil {
			log.Printf("[Upload] Error writing to temp file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process PDF"})
			return
		}

		cmd := exec.Command("pdftotext", tmpFile.Name(), "-")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			log.Printf("[Upload] Error extracting text from PDF: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to extract text from PDF"})
			return
		}

		contentStr = out.String()
		if contentStr == "" {
			log.Printf("[Upload] Warning: No text extracted from PDF")
			contentStr = "PDF document (no text content extracted)"
		}
		log.Printf("[Upload] Extracted %d bytes of text from PDF", len(contentStr))
	} else {
		contentStr = string(content)
	}

	doc := models.Document{
		ID:        models.GenerateID(),
		Path:      file.Filename,
		Type:      ext,
		Content:   contentStr,
		Indexed:   time.Now(),
	}

	if ext == ".pdf" {
		doc.OriginalContent = base64.StdEncoding.EncodeToString(content)
	}

	log.Printf("[Upload] Created document with ID: %s", doc.ID)

	if err := BatchProcessor.AddDocument(doc); err != nil {
		log.Printf("[Upload] Error adding document to batch: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to queue document for indexing: %v", err)})
		return
	}
	
	log.Printf("[Upload] Successfully queued document for indexing: %s", doc.ID)
	
	response := gin.H{
		"message":     "File uploaded and queued for indexing",
		"id":          doc.ID,
		"filename":    file.Filename,
		"size":        file.Size,
		"type":        ext,
		"download_url": fmt.Sprintf("/api/documents/%s/download", doc.ID),
		"view_url":    fmt.Sprintf("/api/documents/%s/view", doc.ID),
	}
	
	log.Printf("[Upload] Sending response: %+v", response)
	c.JSON(http.StatusOK, response)
}

func DownloadDocumentHandler(c *gin.Context) {
	docID := c.Param("id")
	
	if docID == "" {
		log.Printf("[Download] Empty document ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Document ID is required"})
		return
	}
	
	log.Printf("[Download] Processing download request for document: %s", docID)
	
	req := esapi.GetRequest{
		Index:      "documents",
		DocumentID: docID,
	}
	
	res, err := req.Do(context.Background(), elasticsearch.Client)
	if err != nil {
		log.Printf("[Download] Error getting document: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting document: " + err.Error()})
		return
	}
	defer res.Body.Close()
	
	if res.StatusCode == 404 {
		log.Printf("[Download] Document not found: %s", docID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}
	
	var result struct {
		Found  bool `json:"found"`
		Source models.Document `json:"_source"`
	}
	
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		log.Printf("[Download] Error decoding response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error decoding response: " + err.Error()})
		return
	}
	
	if !result.Found {
		log.Printf("[Download] Document not found in response: %s", docID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}

	var content []byte
	if strings.ToLower(result.Source.Type) == ".pdf" {
		if result.Source.OriginalContent != "" {
			content, err = base64.StdEncoding.DecodeString(result.Source.OriginalContent)
		} else {
			content, err = base64.StdEncoding.DecodeString(result.Source.Content)
		}
		if err != nil {
			log.Printf("[Download] Error decoding PDF content: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error decoding PDF content"})
			return
		}
	} else {
		content = []byte(result.Source.Content)
	}
	
	fileName := filepath.Base(result.Source.Path)
	contentType := "text/plain"
	
	switch strings.ToLower(result.Source.Type) {
	case ".pdf":
		contentType = "application/pdf"
	case ".doc":
		contentType = "application/msword"
	case ".docx":
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Header("Content-Type", contentType)
	c.Data(http.StatusOK, contentType, content)
	
	log.Printf("[Download] Successfully sent document: %s", docID)
}

func ViewDocumentHandler(c *gin.Context) {
	docID := c.Param("id")
	
	if docID == "" {
		log.Printf("[View] Empty document ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Document ID is required"})
		return
	}
	
	log.Printf("[View] Processing view request for document: %s", docID)
	
	req := esapi.GetRequest{
		Index:      "documents",
		DocumentID: docID,
	}
	
	res, err := req.Do(context.Background(), elasticsearch.Client)
	if err != nil {
		log.Printf("[View] Error getting document: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting document: " + err.Error()})
		return
	}
	defer res.Body.Close()
	
	if res.StatusCode == 404 {
		log.Printf("[View] Document not found: %s", docID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}
	
	var result struct {
		Found  bool `json:"found"`
		Source models.Document `json:"_source"`
	}
	
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		log.Printf("[View] Error decoding response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error decoding response: " + err.Error()})
		return
	}
	
	if !result.Found {
		log.Printf("[View] Document not found in response: %s", docID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}

	var content []byte
	if strings.ToLower(result.Source.Type) == ".pdf" {
		if result.Source.OriginalContent != "" {
			content, err = base64.StdEncoding.DecodeString(result.Source.OriginalContent)
		} else {
			content, err = base64.StdEncoding.DecodeString(result.Source.Content)
		}
		if err != nil {
			log.Printf("[View] Error decoding PDF content: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error decoding PDF content"})
			return
		}
	} else {
		content = []byte(result.Source.Content)
	}
	
	switch strings.ToLower(result.Source.Type) {
	case ".txt":
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
	case ".pdf":
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", "inline; filename="+filepath.Base(result.Source.Path))
		c.Data(http.StatusOK, "application/pdf", content)
	case ".doc", ".docx":
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/api/documents/%s/download", docID))
	default:
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/api/documents/%s/download", docID))
	}
	
	log.Printf("[View] Successfully processed view request for document: %s", docID)
} 