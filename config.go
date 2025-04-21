package main

import (
    "encoding/json"
    "io/ioutil"
)

// Config holds settings for Ollama integration and prompts.
type Config struct {
    OllamaURL string            `json:"ollama_url"`
    Model      string            `json:"model"`
    IgnoreDirs         []string          `json:"ignore_dirs"`
    // Limit for number of context messages saved in history file; when exceeded, oldest 20% are removed.
    ContextFileLimit   int               `json:"context_file_limit"`
    Prompts            map[string]string `json:"prompts"`
}

// LoadConfig reads and parses config from the given path.
func LoadConfig(path string) (*Config, error) {
    data, err := ioutil.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var conf Config
    if err := json.Unmarshal(data, &conf); err != nil {
        return nil, err
    }
    return &conf, nil
}