package types

// Config struct holds configuration information
// (moved from root types.go for proper Go package structure)
type Config struct {
    OllamaURL        string            `json:"ollama_url"`
    Model             string            `json:"model"`
    IgnoreDirs        []string          `json:"ignore_dirs"`
    ContextFileLimit int               `json:"context_file_limit"`
    ActionLimit       int               `json:"action_limit"`
    Prompts           map[string]string `json:"prompts"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type History []Message
