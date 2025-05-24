package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/shallowseek/config"
	"github.com/shallowseek/dict"
)

var Client *elasticsearch.Client

func Init() error {
    maxRetries := 10
    for i := 0; i < maxRetries; i++ {
        var err error
        Client, err = elasticsearch.NewClient(elasticsearch.Config{
            Addresses: []string{config.GetElasticsearchURL()},
        })
        if err != nil {
            log.Printf("Elasticsearch client creation failed (attempt %d/%d): %s", i+1, maxRetries, err)
            time.Sleep(5 * time.Second)
            continue
        }

        synonymsConfig, err := dict.GetSynonymsConfig()
        if err != nil {
            log.Printf("Error loading synonyms (attempt %d/%d): %s", i+1, maxRetries, err)
            time.Sleep(5 * time.Second)
            continue
        }

        settings := map[string]interface{}{
            "settings": map[string]interface{}{
                "analysis": map[string]interface{}{
                    "analyzer": map[string]interface{}{
                        "custom_analyzer": map[string]interface{}{
                            "type":      "custom",
                            "tokenizer": "standard",
                            "filter": []string{
                                "lowercase",
                                "asciifolding",
                                "russian_synonyms",
                                "russian_stop",
                                "english_stop",
                                "english_stemmer",
                                "english_possessive_stemmer",
                                "english_porter_stemmer",
                            },
                        },
                    },
                    "filter": map[string]interface{}{
                        "russian_synonyms": map[string]interface{}{
                            "type":     "synonym",
                            "synonyms": strings.Split(synonymsConfig, "\n"),
                        },
                        "russian_stop": map[string]interface{}{
                            "type":     "stop",
                            "stopwords": "_russian_",
                        },
                        "english_stop": map[string]interface{}{
                            "type":     "stop",
                            "stopwords": "_english_",
                        },
                        "english_stemmer": map[string]interface{}{
                            "type":     "stemmer",
                            "language": "english",
                        },
                        "english_possessive_stemmer": map[string]interface{}{
                            "type":     "stemmer",
                            "language": "possessive_english",
                        },
                        "english_porter_stemmer": map[string]interface{}{
                            "type":     "stemmer",
                            "language": "porter2",
                        },
                    },
                },
            },
            "mappings": map[string]interface{}{
                "properties": map[string]interface{}{
                    "id": map[string]interface{}{
                        "type": "keyword",
                    },
                    "path": map[string]interface{}{
                        "type": "text",
                        "analyzer": "custom_analyzer",
                        "fields": map[string]interface{}{
                            "keyword": map[string]interface{}{
                                "type":         "keyword",
                                "ignore_above": 256,
                            },
                        },
                    },
                    "type": map[string]interface{}{
                        "type": "keyword",
                    },
                    "content": map[string]interface{}{
                        "type":            "text",
                        "analyzer":        "custom_analyzer",
                        "search_analyzer": "custom_analyzer",
                        "term_vector":     "with_positions_offsets",
                        "index_options":   "positions",
                        "fields": map[string]interface{}{
                            "keyword": map[string]interface{}{
                                "type":         "keyword",
                                "ignore_above": 256,
                            },
                            "elser": map[string]interface{}{
                                "type": "text",
                                "analyzer": "standard",
                            },
                        },
                    },
                    "original_content": map[string]interface{}{
                        "type": "binary",
                    },
                    "indexed": map[string]interface{}{
                        "type": "date",
                    },
                },
            },
        }

        settingsJSON, err := json.Marshal(settings)
        if err != nil {
            log.Printf("Error encoding settings to JSON (attempt %d/%d): %s", i+1, maxRetries, err)
            time.Sleep(5 * time.Second)
            continue
        }

        res, err := Client.Indices.Create(
            "documents",
            Client.Indices.Create.WithBody(strings.NewReader(string(settingsJSON))),
        )
        if err != nil {
            log.Printf("Error creating index (attempt %d/%d): %s", i+1, maxRetries, err)
            time.Sleep(5 * time.Second)
            continue
        }
        defer res.Body.Close()

        if res.IsError() {
            log.Printf("Error creating index (attempt %d/%d): %s", i+1, maxRetries, res.String())
            time.Sleep(5 * time.Second)
            continue
        }

        log.Println("Successfully created index with custom analyzer")
        return nil
    }

    return fmt.Errorf("failed to create index after %d attempts", maxRetries)
}

func GetClusterHealth() (map[string]interface{}, error) {
    res, err := Client.Cluster.Health(
        Client.Cluster.Health.WithContext(context.Background()),
    )
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()

    var health map[string]interface{}
    if err := json.NewDecoder(res.Body).Decode(&health); err != nil {
        return nil, err
    }

    return health, nil
}

func GetIndexStatus() (map[string]interface{}, error) {
    res, err := Client.Indices.Stats(
        Client.Indices.Stats.WithIndex("documents"),
    )
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()

    var stats map[string]interface{}
    if err := json.NewDecoder(res.Body).Decode(&stats); err != nil {
        return nil, err
    }

    return stats, nil
}

func GetDocumentCount() (int64, error) {
    res, err := Client.Count(
        Client.Count.WithIndex("documents"),
    )
    if err != nil {
        return 0, err
    }
    defer res.Body.Close()

    var count map[string]interface{}
    if err := json.NewDecoder(res.Body).Decode(&count); err != nil {
        return 0, err
    }

    if countVal, ok := count["count"].(float64); ok {
        return int64(countVal), nil
    }

    return 0, fmt.Errorf("invalid count response format")
} 