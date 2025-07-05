package schema

import (
	"strings"
	"testing"
)

// MockSchemaFile implements SchemaFile interface for testing
type MockSchemaFile struct {
	name    string
	content string
}

func (m *MockSchemaFile) Name() string {
	return m.name
}

func (m *MockSchemaFile) Read() ([]byte, error) {
	return []byte(m.content), nil
}

func TestParseCommandFromMarkdown_BasicCommand(t *testing.T) {
	content := `# Description

This is a basic command for testing.

# Options
` + "```json" + `
[
    {
        "flags": "--verbose",
        "description": "Enable verbose output",
        "type": "boolean"
    }
]
` + "```" + `

# Arguments
` + "```json" + `
[
    {
        "name": "input",
        "description": "Input file path",
        "type": "string",
        "default": ""
    }
]
` + "```" + `

# Settings
` + "```json" + `
{
    "name": "test-cmd"
}
` + "```"

	file := &MockSchemaFile{name: "test.md", content: content}
	cmd, err := parseCommandFromMarkdown(file, "default-name")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check command name (should be overridden by settings)
	if cmd.Name != "test-cmd" {
		t.Errorf("Expected command name 'test-cmd', got '%s'", cmd.Name)
	}

	// Check description (should come from Description section)
	if cmd.Description != "This is a basic command for testing." {
		t.Errorf("Expected description 'This is a basic command for testing.', got '%s'", cmd.Description)
	}

	// Check options
	if len(cmd.Options) != 1 {
		t.Fatalf("Expected 1 option, got %d", len(cmd.Options))
	}
	if cmd.Options[0].Flags != "--verbose" {
		t.Errorf("Expected option flag '--verbose', got '%s'", cmd.Options[0].Flags)
	}

	// Check arguments
	if len(cmd.Arguments) != 1 {
		t.Fatalf("Expected 1 argument, got %d", len(cmd.Arguments))
	}
	if cmd.Arguments[0].Name != "input" {
		t.Errorf("Expected argument name 'input', got '%s'", cmd.Arguments[0].Name)
	}
}

func TestParseCommandFromMarkdown_CaseInsensitiveHeaders(t *testing.T) {
	content := `# description

This is a test with mixed case headers.

# OPTIONS
` + "```json" + `
[
    {
        "flags": "--debug",
        "description": "Enable debug mode",
        "type": "boolean"
    }
]
` + "```" + `

# Arguments
` + "```json" + `
[]
` + "```" + `

# SETTINGS
` + "```json" + `
{
    "name": "mixed-case-cmd"
}
` + "```"

	file := &MockSchemaFile{name: "mixed.md", content: content}
	cmd, err := parseCommandFromMarkdown(file, "default-name")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check that case-insensitive headers work
	if cmd.Name != "mixed-case-cmd" {
		t.Errorf("Expected command name 'mixed-case-cmd', got '%s'", cmd.Name)
	}

	if cmd.Description != "This is a test with mixed case headers." {
		t.Errorf("Expected description from lowercase 'description' header, got '%s'", cmd.Description)
	}

	if len(cmd.Options) != 1 {
		t.Fatalf("Expected 1 option from 'OPTIONS' header, got %d", len(cmd.Options))
	}
}

func TestParseCommandFromMarkdown_DescriptionPriority(t *testing.T) {
	content := `# Description

Description from dedicated section.

# Settings
` + "```json" + `
{
    "name": "priority-test",
    "description": "Description from settings"
}
` + "```"

	file := &MockSchemaFile{name: "priority.md", content: content}
	cmd, err := parseCommandFromMarkdown(file, "default-name")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Description section should take priority over settings
	if cmd.Description != "Description from dedicated section." {
		t.Errorf("Expected description from Description section, got '%s'", cmd.Description)
	}
}

func TestParseCommandFromMarkdown_SettingsDescriptionFallback(t *testing.T) {
	content := `# Settings
` + "```json" + `
{
    "name": "fallback-test",
    "description": "Description from settings only"
}
` + "```"

	file := &MockSchemaFile{name: "fallback.md", content: content}
	cmd, err := parseCommandFromMarkdown(file, "default-name")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should use description from settings when no Description section
	if cmd.Description != "Description from settings only" {
		t.Errorf("Expected description from settings, got '%s'", cmd.Description)
	}
}

func TestParseCommandFromMarkdown_EmptyFile(t *testing.T) {
	content := ``

	file := &MockSchemaFile{name: "empty.md", content: content}
	cmd, err := parseCommandFromMarkdown(file, "default-name")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should use default name when no settings
	if cmd.Name != "default-name" {
		t.Errorf("Expected default name 'default-name', got '%s'", cmd.Name)
	}

	// Should have empty description
	if cmd.Description != "" {
		t.Errorf("Expected empty description, got '%s'", cmd.Description)
	}

	// Should have empty slices
	if len(cmd.Options) != 0 {
		t.Errorf("Expected 0 options, got %d", len(cmd.Options))
	}
	if len(cmd.Arguments) != 0 {
		t.Errorf("Expected 0 arguments, got %d", len(cmd.Arguments))
	}
}

func TestParseCommandFromMarkdown_MultilineDescription(t *testing.T) {
	content := `# Description

This is a multiline description.

It spans multiple paragraphs and should be 
cleaned up properly.

# Settings
` + "```json" + `
{
    "name": "multiline-test"
}
` + "```"

	file := &MockSchemaFile{name: "multiline.md", content: content}
	cmd, err := parseCommandFromMarkdown(file, "default-name")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "This is a multiline description. It spans multiple paragraphs and should be cleaned up properly."
	if cmd.Description != expected {
		t.Errorf("Expected cleaned multiline description, got '%s'", cmd.Description)
	}
}

func TestParseCommandFromMarkdown_InvalidJSON(t *testing.T) {
	content := `# Options
` + "```json" + `
[
    {
        "flags": "--invalid"
        "missing_comma": true
    }
]
` + "```"

	file := &MockSchemaFile{name: "invalid.md", content: content}
	_, err := parseCommandFromMarkdown(file, "default-name")

	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}

	// Should contain "failed to parse options JSON" in error message
	if !contains(err.Error(), "failed to parse options JSON") {
		t.Errorf("Expected JSON parsing error, got '%s'", err.Error())
	}
}

func TestParseCommandFromMarkdown_ComplexExample(t *testing.T) {
	content := `# Description

A complex command with multiple options and arguments.

# Options
` + "```json" + `
[
    {
        "flags": "--output",
        "description": "Output file path",
        "type": "string",
        "default": "output.txt"
    },
    {
        "flags": "--verbose",
        "description": "Enable verbose logging",
        "type": "boolean"
    }
]
` + "```" + `

# Arguments
` + "```json" + `
[
    {
        "name": "source",
        "description": "Source file or directory",
        "type": "string",
        "default": ""
    },
    {
        "name": "destination",
        "description": "Destination path",
        "type": "string",
        "default": ""
    }
]
` + "```" + `

# Settings
` + "```json" + `
{
    "name": "complex-cmd"
}
` + "```"

	file := &MockSchemaFile{name: "complex.md", content: content}
	cmd, err := parseCommandFromMarkdown(file, "default-name")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify all components
	if cmd.Name != "complex-cmd" {
		t.Errorf("Expected name 'complex-cmd', got '%s'", cmd.Name)
	}

	if cmd.Description != "A complex command with multiple options and arguments." {
		t.Errorf("Expected complex description, got '%s'", cmd.Description)
	}

	if len(cmd.Options) != 2 {
		t.Fatalf("Expected 2 options, got %d", len(cmd.Options))
	}

	if len(cmd.Arguments) != 2 {
		t.Fatalf("Expected 2 arguments, got %d", len(cmd.Arguments))
	}

	// Check specific option details
	outputOption := cmd.Options[0]
	if outputOption.Flags != "--output" || outputOption.Default != "output.txt" {
		t.Errorf("Output option not parsed correctly: %+v", outputOption)
	}

	// Check specific argument details
	sourceArg := cmd.Arguments[0]
	if sourceArg.Name != "source" || sourceArg.Type != "string" {
		t.Errorf("Source argument not parsed correctly: %+v", sourceArg)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
