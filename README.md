# Open Coder
Lightweight coding agent that runs in your terminal and use open-source models

## Installation
```bash
go build -o open-coder main.go
```

## Configuration
Update config.json
Update:
- ollama_url
- model

## Models
- `qwq`
- `llama2-chat`

## Usage
```bash
./open-coder
```

### Add alias to your shell
 ```bash
nano ~/.bashrc
```
or
```bash
nano ~/.zshrc
```
Add the following line at the end of the file:
```bash
alias oc='/full/path/to/your/open-coder-command'
```
Save the file and exit the editor. Then, run the following command to apply the changes:
```bash
source ~/.bashrc
```
or
```bash
source ~/.zshrc
```