package markjson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseV2(t *testing.T) {
	tests := []struct {
		name         string
		markdownFile string
		jsonFile     string
	}{
		{
			name:         "Simple text section",
			markdownFile: "simple-text.md",
			jsonFile:     "simple-text.json",
		},
		{
			name:         "Section with code block",
			markdownFile: "section-with-code.md",
			jsonFile:     "section-with-code.json",
		},
		{
			name:         "Multiple sections with mixed content",
			markdownFile: "multiple-sections.md",
			jsonFile:     "multiple-sections.json",
		},
		{
			name:         "Section with no language code block",
			markdownFile: "no-language-code.md",
			jsonFile:     "no-language-code.json",
		},
		{
			name:         "Empty content",
			markdownFile: "empty.md",
			jsonFile:     "empty.json",
		},
		{
			name:         "Content without sections",
			markdownFile: "no-sections.md",
			jsonFile:     "no-sections.json",
		},
		{
			name:         "Section with only whitespace",
			markdownFile: "whitespace-only.md",
			jsonFile:     "whitespace-only.json",
		},
		{
			name:         "Nested headers",
			markdownFile: "nested-headers.md",
			jsonFile:     "nested-headers.json",
		},
	}

	for _, tt := range tests {
		name := tt.name
		if tt.name == "" {
			name = tt.markdownFile
		}
		t.Run(name, func(t *testing.T) {
			markdownContent, err := os.ReadFile(filepath.Join("testdata", tt.markdownFile))
			if err != nil {
				t.Fatalf("Failed to read markdown file %s: %v", tt.markdownFile, err)
			}
			// Parse the markdown content
			sections, err := Parse(string(markdownContent))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			// Load expected results from JSON file
			expected := loadExpectedSections(t, tt.jsonFile)

			// Compare results
			compareSections(t, sections, expected)
		})
	}
}

func TestParseV2_EdgeCases(t *testing.T) {
	t.Run("Multiple consecutive code blocks", func(t *testing.T) {
		markdownContent, err := os.ReadFile(filepath.Join("testdata", "multiple-code-blocks.md"))
		if err != nil {
			t.Fatalf("Failed to read markdown file %s: %v", "multiple-code-blocks.md", err)
		}
		sections, err := Parse(string(markdownContent))
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		expected := loadExpectedSections(t, "multiple-code-blocks.json")
		compareSections(t, sections, expected)
	})

	t.Run("Unclosed code block", func(t *testing.T) {
		markdownContent, err := os.ReadFile(filepath.Join("testdata", "unclosed-code.md"))
		if err != nil {
			t.Fatalf("Failed to read markdown file %s: %v", "unclosed-code.md", err)
		}
		sections, err := Parse(string(markdownContent))
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		expected := loadExpectedSections(t, "unclosed-code.json")
		compareSections(t, sections, expected)
	})
}

// loadExpectedSections loads expected sections from a JSON file
func loadExpectedSections(t *testing.T, filename string) []*Section {
	t.Helper()

	jsonPath := filepath.Join("testdata", filename)
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to read expected JSON file %s: %v", jsonPath, err)
	}

	var expected []*Section
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("Failed to unmarshal expected JSON from %s: %v", jsonPath, err)
	}

	return expected
}

// compareSections compares actual and expected sections
func compareSections(t *testing.T, actual, expected []*Section) {
	t.Helper()

	if len(actual) != len(expected) {
		t.Errorf("Parse() returned %d sections, expected %d", len(actual), len(expected))
		return
	}

	for i, section := range actual {
		expectedSection := expected[i]

		if section.Title != expectedSection.Title {
			t.Errorf("Section %d title = %q, expected %q", i, section.Title, expectedSection.Title)
		}

		if len(section.Snippets) != len(expectedSection.Snippets) {
			t.Errorf("Section %d has %d snippets, expected %d", i, len(section.Snippets), len(expectedSection.Snippets))
			continue
		}

		for j, snippet := range section.Snippets {
			expectedSnippet := expectedSection.Snippets[j]

			if snippet.Type != expectedSnippet.Type {
				t.Errorf("Section %d, snippet %d type = %q, expected %q", i, j, snippet.Type, expectedSnippet.Type)
			}

			if snippet.Language != expectedSnippet.Language {
				t.Errorf("Section %d, snippet %d language = %q, expected %q", i, j, snippet.Language, expectedSnippet.Language)
			}

			if snippet.Content != expectedSnippet.Content {
				t.Errorf("Section %d, snippet %d content = %q, expected %q", i, j, snippet.Content, expectedSnippet.Content)
			}
		}
	}
}
