package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Message represents a single conversation entry with role and content.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func main() {
	configPath := flag.String("config", "config.json", "Path to config file")
	promptKey := flag.String("prompt", "default", "Prompt template key in config")
	flag.Parse()

	conf, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Gather static environment context for LLM
	osName := runtime.GOOS
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get current directory: %v\n", err)
		os.Exit(1)
	}
	// Prepare ignore directories map
	ignoreMap := make(map[string]bool)
	for _, d := range conf.IgnoreDirs {
		ignoreMap[d] = true
	}
	// Static system prompt without file listing (file state will be added per interaction)
	staticPrompt := fmt.Sprintf(
		"Environment:\nOS: %s\nWorking directory: %s\nIgnore directories: %s\n",
		osName, cwd, strings.Join(conf.IgnoreDirs, ", "),
	)
	// Setup context history directory, snapshot path, and file
	contextDir := filepath.Join(cwd, ".open-coder")
	snapshotPath := filepath.Join(contextDir, "fs_snapshot.txt")
	historyPath := filepath.Join(contextDir, "history.json")
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create context directory: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		if err := ioutil.WriteFile(historyPath, []byte("[]"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create history file: %v\n", err)
			os.Exit(1)
		}
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		// Compute current file system listing for context and snapshot
		var dirs []string
		filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			base := filepath.Base(path)
			if ignoreMap[base] {
				return fs.SkipDir
			}
			dirs = append(dirs, path)
			return nil
		})
		sort.Strings(dirs)
		var listingBuilder strings.Builder
		listingBuilder.WriteString("File system listing (ls -R):\n")
		for _, dir := range dirs {
			listingBuilder.WriteString(dir + ":\n")
			entries, err := ioutil.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				name := entry.Name()
				if ignoreMap[name] {
					continue
				}
				listingBuilder.WriteString("  " + name + "\n")
			}
			listingBuilder.WriteString("\n")
		}
		fsListing := listingBuilder.String()
		// Save snapshot to file
		if err := ioutil.WriteFile(snapshotPath, []byte(fsListing), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write file system snapshot: %v\n", err)
		}
		// Compose full system prompt with file state
		systemPrompt := staticPrompt + fsListing
		fmt.Print("> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println()
				return
			}
			fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			os.Exit(1)
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		{
			type CommandResponse struct {
				Commands    []string `json:"commands"`
				Explanation string   `json:"explanation"`
			}

			var crDirect CommandResponse
			if err := json.Unmarshal([]byte(input), &crDirect); err == nil && len(crDirect.Commands) > 0 {
				fmt.Println("Executing commands:")
				for _, cmdStr := range crDirect.Commands {
					fmt.Printf("> %s\n", cmdStr)
					cmd := exec.Command("bash", "-lc", cmdStr)
					cmd.Dir = cwd
					out, err := cmd.CombinedOutput()
					if len(out) > 0 {
						fmt.Printf("%s\n", out)
					} else {
						fmt.Println("(no output)")
					}
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error executing command '%s': %v\n", cmdStr, err)
					}
				}
				fmt.Println("Done executing commands.")
				if crDirect.Explanation != "" {
					fmt.Println("Explanation:")
					fmt.Println(crDirect.Explanation)
				}
				// Skip LLM interaction and continue loop
				continue
			}
		}
		if input == "/clear" {
			if err := ioutil.WriteFile(historyPath, []byte("[]"), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to clear history: %v\n", err)
			} else {
				fmt.Println("Context history cleared.")
			}
			continue
		}
		if input == "exit" || input == "quit" {
			fmt.Println("Bye!")
			return
		}

		tmpl, ok := conf.Prompts[*promptKey]
		if !ok {
			fmt.Fprintf(os.Stderr, "Prompt '%s' not found in config\n", *promptKey)
			os.Exit(1)
		}
		// Build and send structured request
		prompt := strings.ReplaceAll(tmpl, "{{input}}", input)
		// Load conversation history
		histBytes, err := ioutil.ReadFile(historyPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read history file: %v\n", err)
			continue
		}
		var history []Message
		if err := json.Unmarshal(histBytes, &history); err != nil {
			history = []Message{}
		}
		// Assemble messages: system prompt, history, and new user message
		var messages []map[string]string
		messages = append(messages, map[string]string{"role": "system", "content": systemPrompt})
		for _, m := range history {
			messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
		}
		messages = append(messages, map[string]string{"role": "user", "content": prompt})
		payload := map[string]interface{}{
			"model":    conf.Model,
			"messages": messages,
			"stream":   false,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal payload: %v\n", err)
			continue
		}
		url := strings.TrimRight(conf.OllamaURL, "/") + "/api/chat"
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create request: %v\n", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			fmt.Fprintf(os.Stderr, "Non-OK HTTP status: %s\n%s\n", resp.Status, string(body))
			continue
		}
		// Read full response body
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading response: %v\n", err)
			continue
		}
		var respObj map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &respObj); err != nil {
			fmt.Println(string(bodyBytes))
			continue
		}
		// Extract assistant content
		var content string
		if choices, ok := respObj["choices"].([]interface{}); ok && len(choices) > 0 {
			if ch0, ok := choices[0].(map[string]interface{}); ok {
				if msg, ok := ch0["message"].(map[string]interface{}); ok {
					if c, ok := msg["content"].(string); ok {
						content = c
					}
				}
			}
		} else if message, ok := respObj["message"].(map[string]interface{}); ok {
			if c, ok := message["content"].(string); ok {
				content = c
			}
		}
		if content == "" {
			fmt.Println(string(bodyBytes))
			continue
		}
		// Parse structured command response
		type CommandResponse struct {
			Commands    []string `json:"commands"`
			Explanation string   `json:"explanation"`
		}
		var cr CommandResponse

		// Step 1: discard the thinking part (if any)
		processed := content
		if idx := strings.LastIndex(processed, "</think>"); idx != -1 {
			processed = processed[idx+len("</think>"):]
		}

		processed = strings.TrimSpace(processed)

		// Step 2: drop Markdown triple‑backtick fences
		if strings.HasPrefix(processed, "```") {
			// Drop the opening fence and optional language hint
			processed = strings.TrimPrefix(processed, "```")
			if nl := strings.Index(processed, "\n"); nl != -1 {
				processed = processed[nl+1:]
			}
			// Drop the closing fence (last occurrence)
			if idx := strings.LastIndex(processed, "```"); idx != -1 {
				processed = processed[:idx]
			}
		}

		// Step 3: slice out the JSON object boundaries
		jsonContent := processed
		if i := strings.Index(processed, "{"); i != -1 {
			if j := strings.LastIndex(processed, "}"); j != -1 && j > i {
				jsonContent = processed[i : j+1]
			}
		}
		// Attempt to parse structured command response
		if err := json.Unmarshal([]byte(jsonContent), &cr); err != nil {
			// Fallback: print raw content
			fmt.Println(content)
			continue
		}
		// Execute commands if any
		if len(cr.Commands) > 0 {
			fmt.Println("Executing commands:")
			for _, cmdStr := range cr.Commands {
				fmt.Printf("> %s\n", cmdStr)
				cmd := exec.Command("bash", "-lc", cmdStr)
				cmd.Dir = cwd
				out, err := cmd.CombinedOutput()
				if len(out) > 0 {
					fmt.Printf("%s\n", out)
				} else {
					fmt.Println("(no output)")
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error executing command '%s': %v\n", cmdStr, err)
				}
			}
			fmt.Println("Done executing commands.")
		}
		// Print explanation
		fmt.Println("Explanation:")
		fmt.Println(cr.Explanation)
		// Append user and assistant messages to history
		history = append(history, Message{Role: "user", Content: prompt}, Message{Role: "assistant", Content: content})
		// Trim history if exceeds limit
		if conf.ContextFileLimit > 0 && len(history) > conf.ContextFileLimit {
			dropCount := int(float64(len(history)) * 0.2)
			if dropCount < 1 {
				dropCount = 1
			}
			history = history[dropCount:]
		}
		newHistBytes, err := json.Marshal(history)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal history: %v\n", err)
		} else if err := ioutil.WriteFile(historyPath, newHistBytes, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write history file: %v\n", err)
		}
	}
}
