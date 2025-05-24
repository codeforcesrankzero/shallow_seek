package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/shallowseek/elasticsearch"
	"github.com/shallowseek/metrics"
	"github.com/shallowseek/models"
)

const (
	batchSize = 100
	timeout   = 30 * time.Second
	flushInterval = 5 * time.Second
)

type BatchProcessor struct {
	documents []models.Document
	mu        sync.Mutex
	stopChan  chan struct{}
}

func NewBatchProcessor() *BatchProcessor {
	bp := &BatchProcessor{
		documents: make([]models.Document, 0, batchSize),
		stopChan:  make(chan struct{}),
	}
	
	go bp.periodicFlush()
	
	return bp
}

func (bp *BatchProcessor) periodicFlush() {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := bp.Flush(); err != nil {
				log.Printf("[Batch] Error during periodic flush: %v", err)
			}
		case <-bp.stopChan:
			if err := bp.Flush(); err != nil {
				log.Printf("[Batch] Error during final flush: %v", err)
			}
			return
		}
	}
}

func (bp *BatchProcessor) AddDocument(doc models.Document) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	log.Printf("[Batch] Adding document %s to batch (current size: %d)", doc.ID, len(bp.documents))
	
	if doc.ID == "" {
		return fmt.Errorf("document ID cannot be empty")
	}
	if doc.Content == "" {
		return fmt.Errorf("document content cannot be empty")
	}
	
	bp.documents = append(bp.documents, doc)

	if len(bp.documents) >= batchSize {
		log.Printf("[Batch] Batch size reached %d, flushing...", batchSize)
		return bp.Flush()
	}

	return nil
}

func (bp *BatchProcessor) Flush() error {
	bp.mu.Lock()
	if len(bp.documents) == 0 {
		bp.mu.Unlock()
		return nil
	}

	docs := bp.documents
	bp.documents = make([]models.Document, 0, batchSize)
	bp.mu.Unlock()

	log.Printf("[Batch] Flushing batch of %d documents", len(docs))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var buf strings.Builder
	for _, doc := range docs {
		buf.WriteString(`{"index":{"_index":"documents","_id":"` + doc.ID + `"}}` + "\n")
		
		docJSON, err := json.Marshal(doc)
		if err != nil {
			log.Printf("[Batch] Error marshaling document %s: %v", doc.ID, err)
			return fmt.Errorf("failed to marshal document %s: %v", doc.ID, err)
		}
		buf.WriteString(string(docJSON) + "\n")
	}

	res, err := elasticsearch.Client.Bulk(
		strings.NewReader(buf.String()),
		elasticsearch.Client.Bulk.WithContext(ctx),
		elasticsearch.Client.Bulk.WithRefresh("true"),
	)
	if err != nil {
		log.Printf("[Batch] Error executing bulk request: %v", err)
		return fmt.Errorf("failed to execute bulk request: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var raw map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
			log.Printf("[Batch] Error decoding error response: %v", err)
			return fmt.Errorf("bulk indexing failed: %s", res.String())
		}

		if errors, ok := raw["errors"].(bool); ok && errors {
			if items, ok := raw["items"].([]interface{}); ok {
				for i, item := range items {
					if index, ok := item.(map[string]interface{})["index"].(map[string]interface{}); ok {
						if err, ok := index["error"].(map[string]interface{}); ok {
							log.Printf("[Batch] Document %d indexing error: %v", i, err)
						}
					}
				}
			}
		}
		return fmt.Errorf("bulk indexing failed: %s", res.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		log.Printf("[Batch] Error decoding success response: %v", err)
		return fmt.Errorf("failed to decode bulk response: %v", err)
	}

	if items, ok := response["items"].([]interface{}); ok {
		for i, item := range items {
			if index, ok := item.(map[string]interface{})["index"].(map[string]interface{}); ok {
				if err, ok := index["error"].(map[string]interface{}); ok {
					log.Printf("[Batch] Document %d indexing error: %v", i, err)
				}
			}
		}
	}

	count, err := elasticsearch.GetDocumentCount()
	if err != nil {
		log.Printf("[Batch] Error getting document count: %v", err)
	} else {
		metrics.DocumentCount.Set(float64(count))
		log.Printf("[Batch] Updated document count to %d", count)
	}

	log.Printf("[Batch] Successfully indexed %d documents", len(docs))
	return nil
}


func (bp *BatchProcessor) Stop() {
	close(bp.stopChan)
} 