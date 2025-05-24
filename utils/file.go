package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func CalculateContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

func ExtractTextFromFile(filePath string) (string, error) {
    ext := strings.ToLower(filepath.Ext(filePath))
    
    switch ext {
    case ".txt":
        content, err := os.ReadFile(filePath)
        if err != nil {
            return "", err
        }
        return string(content), nil
        
    case ".pdf":
        cmd := exec.Command("pdftotext", filePath, "-")
        var out bytes.Buffer
        cmd.Stdout = &out
        err := cmd.Run()
        if err != nil {
            return "", fmt.Errorf("error extracting text from PDF: %v", err)
        }
        return out.String(), nil
        
    case ".doc":
        cmd := exec.Command("antiword", filePath)
        var out bytes.Buffer
        cmd.Stdout = &out
        err := cmd.Run()
        if err != nil {
            return "", fmt.Errorf("error extracting text from DOC: %v", err)
        }
        return out.String(), nil
        
    case ".docx":
        cmd := exec.Command("docx2txt", filePath, "-")
        var out bytes.Buffer
        cmd.Stdout = &out
        err := cmd.Run()
        if err != nil {
            return "", fmt.Errorf("error extracting text from DOCX: %v", err)
        }
        return out.String(), nil
        
    default:
        return "", fmt.Errorf("unsupported file format: %s", ext)
    }
}

func EnsureDirectoryExists(dir string) error {
    if _, err := os.Stat(dir); os.IsNotExist(err) {
        return os.MkdirAll(dir, 0755)
    }
    return nil
} 