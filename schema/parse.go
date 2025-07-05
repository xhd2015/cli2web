package schema

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xhd2015/cli2web/config"
	"github.com/xhd2015/cli2web/markjson"
)

// parseCommandFromMarkdown parses a markdown file to extract command definition
func parseCommandFromMarkdown(file SchemaFile, defaultName string) (*config.Command, error) {
	content, err := file.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", file.Name(), err)
	}

	cmd := &config.Command{
		Name: defaultName,
	}

	// Parse the markdown content
	sections, err := markjson.Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse markdown content: %w", err)
	}

	// Parse description from dedicated section first
	if section := sections.Find("description"); section != nil {
		// Description section contains plain text, not JSON
		rawDescription := strings.TrimSpace(section.Snippets.CombineAllTexts())
		// Clean up the description by normalizing whitespace
		cmd.Description = cleanupDescription(rawDescription)
	}

	// Parse options
	if section := sections.Find("options"); section != nil {
		var options []*config.Option
		if jsonSnippet := section.Snippets.FindJson(); jsonSnippet != nil {
			if err := json.Unmarshal([]byte(jsonSnippet.Content), &options); err != nil {
				return nil, fmt.Errorf("failed to parse options JSON: %w", err)
			}
			cmd.Options = options
		}
	}

	// Parse arguments
	if section := sections.Find("arguments"); section != nil {
		var arguments []*config.Argument
		if jsonSnippet := section.Snippets.FindJson(); jsonSnippet != nil {
			if err := json.Unmarshal([]byte(jsonSnippet.Content), &arguments); err != nil {
				return nil, fmt.Errorf("failed to parse arguments JSON: %w", err)
			}
			cmd.Arguments = arguments
		}
	}

	if section := sections.Find("examples"); section != nil {
		var examples []*config.Example

		n := len(section.Snippets)
		var descriptions []string
		for i := 0; i < n; i++ {
			snippet := section.Snippets[i]
			if snippet.Type != markjson.Code {
				descriptions = append(descriptions, snippet.Content)
				continue
			}
			examples = append(examples, &config.Example{
				Usage:       snippet.Content,
				Description: strings.Join(descriptions, "\n"),
			})
			descriptions = nil
		}
		if len(descriptions) > 0 {
			examples = append(examples, &config.Example{
				Usage:       "",
				Description: strings.Join(descriptions, "\n"),
			})
		}
		cmd.Examples = examples
	}

	// Parse settings
	if section := sections.Find("settings"); section != nil {
		var settings map[string]interface{}
		if jsonSnippet := section.Snippets.FindJson(); jsonSnippet != nil {
			if err := json.Unmarshal([]byte(jsonSnippet.Content), &settings); err != nil {
				return nil, fmt.Errorf("failed to parse settings JSON: %w", err)
			}
		}

		// Override command name if specified in settings
		if name, ok := settings["name"].(string); ok && name != "" {
			cmd.Name = name
		}

		// Set description if specified in settings (only if not already set from description section)
		if cmd.Description == "" {
			if desc, ok := settings["description"].(string); ok {
				cmd.Description = desc
			}
		}
	}

	return cmd, nil
}

// cleanupDescription normalizes whitespace in a description string
func cleanupDescription(desc string) string {
	// Replace multiple whitespace characters (including newlines) with single spaces
	// and trim the result
	return strings.Join(strings.Fields(desc), " ")
}
