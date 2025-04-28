package prompt

import (
	"fmt"
	"github.com/ZanMax/open-coder/types"
	"github.com/ZanMax/open-coder/utils"
)

func LoadPrompt(conf *types.Config, promptKey string) string {
	prompt, ok := conf.Prompts[promptKey]
	if !ok {
		utils.HandleError(fmt.Errorf("Prompt '%s' not found in config", promptKey), "Failed to load prompt")
	}
	return prompt
}