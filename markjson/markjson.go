package markjson

import (
	"strings"
)

type Section struct {
	// e.g. Arguments
	Title    string   `json:"title"`
	Snippets Snippets `json:"snippets"`
}

type SnippetType string

const (
	Text SnippetType = "text"
	Code SnippetType = "code"
)

type Snippet struct {
	Type     SnippetType `json:"type"`
	Language string      `json:"language,omitempty"` // effective when Type == "code", can be json, and other things
	Content  string      `json:"content"`
}

type Sections []*Section

type Snippets []*Snippet

// Parse extracts all sections from markdown content
func Parse(content string) (Sections, error) {
	var sections Sections

	// Split content into lines for processing
	lines := strings.Split(content, "\n")

	var currentSection *Section
	var currentSnippets Snippets
	var currentSnippet *Snippet
	var inCodeBlock bool
	var codeBlockLanguage string
	var codeLines []string
	var textLines []string

	for _, line := range lines {
		// Check if this line is a section header (starts with #)
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			// Save previous section if exists
			if currentSection != nil {
				// Add any pending snippet
				if currentSnippet != nil {
					if currentSnippet.Type == Code {
						currentSnippet.Content = strings.Join(codeLines, "\n")
					} else {
						currentSnippet.Content = strings.TrimSpace(strings.Join(textLines, "\n"))
					}
					currentSnippets = append(currentSnippets, currentSnippet)
				} else if len(textLines) > 0 {
					// Handle accumulated text lines without a current snippet
					textContent := strings.TrimSpace(strings.Join(textLines, "\n"))
					if textContent != "" {
						snippet := &Snippet{
							Type:    Text,
							Content: textContent,
						}
						currentSnippets = append(currentSnippets, snippet)
					}
				}

				currentSection.Snippets = currentSnippets
				sections = append(sections, currentSection)
			}

			// Start new section
			title := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
			currentSection = &Section{
				Title:    title,
				Snippets: []*Snippet{},
			}
			currentSnippets = []*Snippet{}
			currentSnippet = nil
			inCodeBlock = false
			codeLines = []string{}
			textLines = []string{}
			continue
		}

		// Skip if no current section
		if currentSection == nil {
			continue
		}

		// Check for code block start/end
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "```") {
			if !inCodeBlock {
				// Starting code block
				// Save any pending text snippet
				if len(textLines) > 0 {
					textContent := strings.TrimSpace(strings.Join(textLines, "\n"))
					if textContent != "" {
						snippet := &Snippet{
							Type:    Text,
							Content: textContent,
						}
						currentSnippets = append(currentSnippets, snippet)
					}
					textLines = []string{}
				}

				// Extract language from code block marker
				codeBlockLanguage = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "```"))
				inCodeBlock = true
				codeLines = []string{}
				currentSnippet = &Snippet{
					Type:     Code,
					Language: codeBlockLanguage,
				}
			} else {
				// Ending code block
				inCodeBlock = false
				if currentSnippet != nil {
					currentSnippet.Content = strings.Join(codeLines, "\n")
					currentSnippets = append(currentSnippets, currentSnippet)
					currentSnippet = nil
				}
				codeLines = []string{}
			}
			continue
		}

		// Collect content based on current state
		if inCodeBlock {
			codeLines = append(codeLines, line)
		} else {
			// Regular text line
			textLines = append(textLines, line)
		}
	}

	// Handle the last section
	if currentSection != nil {
		// Add any pending snippet
		if currentSnippet != nil {
			if currentSnippet.Type == Code {
				currentSnippet.Content = strings.Join(codeLines, "\n")
			} else {
				currentSnippet.Content = strings.TrimSpace(strings.Join(textLines, "\n"))
			}
			currentSnippets = append(currentSnippets, currentSnippet)
		} else if len(textLines) > 0 {
			textContent := strings.TrimSpace(strings.Join(textLines, "\n"))
			if textContent != "" {
				snippet := &Snippet{
					Type:    Text,
					Content: textContent,
				}
				currentSnippets = append(currentSnippets, snippet)
			}
		}

		currentSection.Snippets = currentSnippets
		sections = append(sections, currentSection)
	}

	return sections, nil
}

func (sections Sections) Find(title string) *Section {
	for _, section := range sections {
		if title == section.Title || strings.ToLower(section.Title) == title {
			return section
		}
	}
	return nil
}

func (snippets Snippets) FindJson() *Snippet {
	for _, snippet := range snippets {
		if snippet.Type == Code && snippet.Language == "json" {
			return snippet
		}
	}
	return nil
}

func (snippets Snippets) CombineAllTexts() string {
	var sb strings.Builder
	for _, snippet := range snippets {
		if snippet.Type == Text {
			sb.WriteString(snippet.Content)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
