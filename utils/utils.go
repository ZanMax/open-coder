package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ZanMax/open-coder/types"
)

var (
	HistoryPath, SnapshotPath, Cwd, ContextDir string
)

func init() {
	var err error
	Cwd, err = os.Getwd()
	HandleError(err, "failed to get current working directory")
	ContextDir = filepath.Join(Cwd, ".context")
	// Create context directory if it doesn't exist
	err = os.MkdirAll(ContextDir, 0755)
	HandleError(err, "failed to create context directory")
	HistoryPath = filepath.Join(ContextDir, "history.json")
	SnapshotPath = filepath.Join(ContextDir, "snapshot.txt")
}

func HandleError(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
		//os.Exit(1)
	}
}

func ClearHistory() error {
	emptyHistory := "[]"
	err := ioutil.WriteFile(HistoryPath, []byte(emptyHistory), 0644)
	if err != nil {
		return fmt.Errorf("Failed to clear history: %w", err)
	}
	return nil
}


// LoadConfig reads and parses config from the given path.
func LoadConfig(path string) (*types.Config, error) {
	data, err := ioutil.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &types.Config{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var conf types.Config
	if err := json.Unmarshal(data, &conf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &conf, nil
}

// GetFileSystemListing generates a file system listing string similar to 'ls -R'.
func GetFileSystemListing(ignoreMap map[string]bool) (string, error) {
	var dirs []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip on error
		}
		if !info.IsDir() {
			return nil // Skip files
		}
		base := filepath.Base(path)
		if ignoreMap[base] {
			return filepath.SkipDir // Skip ignored directories
		}
		dirs = append(dirs, path)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("error walking the file system: %w", err)
	}
	sort.Strings(dirs)
	var listingBuilder strings.Builder
	listingBuilder.WriteString("File system listing (ls -R):\n")
	for _, dir := range dirs {
		listingBuilder.WriteString(dir + ":\n")
		entries, err := ioutil.ReadDir(dir)
		if err != nil {
			continue // Skip directory if it can't be read
		}
		for _, entry := range entries {
			name := entry.Name()
			if ignoreMap[name] {
				continue // Skip ignored entries
			}
			listingBuilder.WriteString("  " + name + "\n")
		}
		listingBuilder.WriteString("\n")
	}
	return listingBuilder.String(), nil
}

func ReadFile(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	return string(content), err
}