package dict

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	synonymsCache     map[string][]string
	synonymsCacheTime time.Time
	synonymsMutex     sync.RWMutex
)

type Word struct {
	ID         int      `json:"id"`
	Name       string   `json:"name"`
	Definition string   `json:"definition"`
	Synonyms   []string `json:"synonyms"`
	Antonyms   []string `json:"antonyms"`
	Similars   []string `json:"similars"`
}

type Dictionary struct {
	Wordlist []Word `json:"wordlist"`
}

func LoadSynonyms() (map[string][]string, error) {
	synonymsMutex.RLock()
	if synonymsCache != nil && time.Since(synonymsCacheTime) < 24*time.Hour {
		defer synonymsMutex.RUnlock()
		return synonymsCache, nil
	}
	synonymsMutex.RUnlock()

	url := "https://raw.githubusercontent.com/egorkaru/synonym_dictionary/master/dictionary.json"
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error downloading synonyms: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	if len(body) > 3 && body[0] == 0xEF && body[1] == 0xBB && body[2] == 0xBF {
		body = body[3:]
	}

	var dict Dictionary
	if err := json.Unmarshal(body, &dict); err != nil {
		return nil, fmt.Errorf("error decoding dictionary: %v", err)
	}

	synonyms := make(map[string][]string)
	
	for _, word := range dict.Wordlist {
		mainWord := strings.ToLower(strings.TrimSpace(word.Name))
		if mainWord == "" || !utf8.ValidString(mainWord) {
			continue
		}

		var allSynonyms []string
		allSynonyms = append(allSynonyms, word.Synonyms...)
		allSynonyms = append(allSynonyms, word.Similars...)

		for _, syn := range allSynonyms {
			syn = strings.ToLower(strings.TrimSpace(syn))
			if syn != "" && syn != mainWord && utf8.ValidString(syn) {
				synonyms[mainWord] = append(synonyms[mainWord], syn)
			}
		}

		for _, syn := range allSynonyms {
			syn = strings.ToLower(strings.TrimSpace(syn))
			if syn != "" && syn != mainWord && utf8.ValidString(syn) {
				synonyms[syn] = append(synonyms[syn], mainWord)
			}
		}
	}

	if len(synonyms) == 0 {
		synonyms = map[string][]string{
			"отец": {"папа", "батя", "батюшка", "родитель"},
			"мать": {"мама", "матушка", "родительница"},
			"сын": {"сынок", "сынишка"},
			"дочь": {"дочка", "доченька"},
			"брат": {"братишка", "братик"},
			"сестра": {"сестричка", "сестрица"},
			"дед": {"дедушка", "дедуля"},
			"бабка": {"бабушка", "бабуля"},
			"муж": {"супруг", "благоверный"},
			"жена": {"супруга", "благоверная"},
			"друг": {"приятель", "товарищ"},
			"враг": {"недруг", "противник"},
			"любовь": {"чувство", "привязанность"},
			"ненависть": {"неприязнь", "вражда"},
			"радость": {"веселье", "счастье"},
			"печаль": {"грусть", "тоска"},
			"жизнь": {"существование", "бытие"},
			"смерть": {"кончина", "гибель"},
			"дом": {"жилище", "кров"},
			"работа": {"труд", "дело"},
		}
	}

	synonymsMutex.Lock()
	synonymsCache = synonyms
	synonymsCacheTime = time.Now()
	synonymsMutex.Unlock()

	return synonyms, nil
}

func GetSynonymsConfig() (string, error) {
	synonyms, err := LoadSynonyms()
	if err != nil {
		return "", err
	}

	var synonymLines []string
	seen := make(map[string]bool)

	for _, syns := range synonyms {
		var uniqueSyns []string
		for _, syn := range syns {
			syn = strings.TrimSpace(syn)
			if syn == "" {
				continue
			}
			if !utf8.ValidString(syn) {
				continue
			}
			if !seen[syn] {
				seen[syn] = true
				uniqueSyns = append(uniqueSyns, syn)
			}
		}
		if len(uniqueSyns) > 1 {
			synonymLines = append(synonymLines, strings.Join(uniqueSyns, ","))
		}
	}

	return strings.Join(synonymLines, "\n"), nil
} 