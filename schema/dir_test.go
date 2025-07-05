package schema

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/xhd2015/cli2web/config"
)

func TestParseSchemaFromDir(t *testing.T) {
	// Create a temporary directory with test schema files
	tempDir := t.TempDir()

	// Create test schema structure
	err := createTestSchemaStructure(tempDir)
	if err != nil {
		t.Fatalf("Failed to create test schema structure: %v", err)
	}

	// Debug: List what files were created
	err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(tempDir, path)
		t.Logf("Created: %s (dir: %t)", relPath, info.IsDir())
		return nil
	})
	if err != nil {
		t.Logf("Error walking directory: %v", err)
	}

	// Test ParseSchemaFromDir
	schema, err := ParseSchemaFromDir(tempDir)
	if err != nil {
		t.Fatalf("ParseSchemaFromDir failed: %v", err)
	}

	// Verify the parsed schema
	verifyTestSchema(t, schema)
}

func TestParseSchemaFromDir_NonExistentPath(t *testing.T) {
	// Test with non-existent path
	_, err := ParseSchemaFromDir("/non/existent/path")
	if err == nil {
		t.Fatal("Expected error for non-existent path, got nil")
	}
}

func TestParseSchemaFromEmbed(t *testing.T) {
	// Skip this test as we can't easily create a real embedded FS in tests
	// In real usage, you would have:
	// //go:embed schema-data
	// var schemaFS embed.FS
	// schema, err := ParseSchemaFromEmbed(schemaFS, "schema-data")
	t.Skip("Skipping embed test - requires actual embedded filesystem")
}

func TestParseSchemaFromEmbed_NonExistentPath(t *testing.T) {
	t.Skip("Skipping embed test - requires actual embedded filesystem")
}

func TestParseSchemaFromFS(t *testing.T) {
	// Create a test filesystem using fstest.MapFS
	testFS := createTestMapFS()

	// Test ParseSchemaFromFS
	schema, err := ParseSchemaFromFS(testFS, "test-schema")
	if err != nil {
		t.Fatalf("ParseSchemaFromFS failed: %v", err)
	}

	// Verify the parsed schema
	verifyTestSchema(t, schema)
}

func TestParseSchemaFromFS_WithOSDirFS(t *testing.T) {
	// Create a temporary directory with test schema files
	tempDir := t.TempDir()

	// Create test schema structure
	err := createTestSchemaStructure(tempDir)
	if err != nil {
		t.Fatalf("Failed to create test schema structure: %v", err)
	}

	// Use os.DirFS to create a filesystem
	osFS := os.DirFS(tempDir)

	// Test ParseSchemaFromFS with os.DirFS
	schema, err := ParseSchemaFromFS(osFS, ".")
	if err != nil {
		t.Fatalf("ParseSchemaFromFS with os.DirFS failed: %v", err)
	}

	// Verify the parsed schema (root name should be "root" for "." path)
	verifyTestSchema(t, schema)
}

func TestParseSchemaFromFS_NonExistentPath(t *testing.T) {
	testFS := createTestMapFS()

	// Test with non-existent path
	_, err := ParseSchemaFromFS(testFS, "non-existent")
	if err == nil {
		t.Fatal("Expected error for non-existent path, got nil")
	}
}

func TestParseSchemaFromDir_CorrectStructure(t *testing.T) {
	// Create a temporary directory with the correct schema structure
	tempDir := t.TempDir()

	// Create the correct structure: command.md files in root, subdirs for subcommands
	err := createCorrectSchemaStructure(tempDir)
	if err != nil {
		t.Fatalf("Failed to create correct schema structure: %v", err)
	}

	// Test ParseSchemaFromDir
	schema, err := ParseSchemaFromDir(tempDir)
	if err != nil {
		t.Fatalf("ParseSchemaFromDir failed: %v", err)
	}

	// Verify the parsed schema has the git command with subcommands
	verifyCorrectSchema(t, schema, filepath.Base(tempDir))
}

// Helper function to create test schema structure in a directory
func createTestSchemaStructure(baseDir string) error {
	// Create help command (no subcommands) - this should be found
	helpContent := `# Description

Show help information.

# Arguments
` + "```json" + `
[
    {
        "name": "command",
        "description": "Command to show help for",
        "type": "string",
        "default": ""
    }
]
` + "```" + `

# Settings
` + "```json" + `
{}
` + "```"

	err := os.WriteFile(filepath.Join(baseDir, "help.md"), []byte(helpContent), 0644)
	if err != nil {
		return err
	}

	// Create git directory and files
	gitDir := filepath.Join(baseDir, "git")
	err = os.MkdirAll(gitDir, 0755)
	if err != nil {
		return err
	}

	gitContent := `# Description

Git version control commands.

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

# Settings
` + "```json" + `
{
    "name": "git"
}
` + "```"

	err = os.WriteFile(filepath.Join(gitDir, "git.md"), []byte(gitContent), 0644)
	if err != nil {
		return err
	}

	// Create git status subcommand
	statusDir := filepath.Join(gitDir, "status")
	err = os.MkdirAll(statusDir, 0755)
	if err != nil {
		return err
	}

	statusContent := `# Description

Show the working tree status.

# Options
` + "```json" + `
[
    {
        "flags": "--short",
        "description": "Give the output in short format",
        "type": "boolean"
    }
]
` + "```" + `

# Settings
` + "```json" + `
{}
` + "```"

	err = os.WriteFile(filepath.Join(statusDir, "status.md"), []byte(statusContent), 0644)
	if err != nil {
		return err
	}

	return nil
}

// Helper function to create test MapFS
func createTestMapFS() fs.FS {
	return fstest.MapFS{
		"test-schema/help.md": &fstest.MapFile{
			Data: []byte(`# Description

Show help information.

# Arguments
` + "```json" + `
[
    {
        "name": "command",
        "description": "Command to show help for",
        "type": "string",
        "default": ""
    }
]
` + "```" + `

# Settings
` + "```json" + `
{}
` + "```"),
		},
		// Note: git/git.md should now be found by the improved parsing logic
		// that handles commands in subdirectories
		"test-schema/git/git.md": &fstest.MapFile{
			Data: []byte(`# Description

Git version control commands.

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

# Settings
` + "```json" + `
{
    "name": "git"
}
` + "```"),
		},
		"test-schema/git/status/status.md": &fstest.MapFile{
			Data: []byte(`# Description

Show the working tree status.

# Options
` + "```json" + `
[
    {
        "flags": "--short",
        "description": "Give the output in short format",
        "type": "boolean"
    }
]
` + "```" + `

# Settings
` + "```json" + `
{}
` + "```"),
		},
	}
}

// Helper function to verify the parsed schema structure
func verifyTestSchema(t *testing.T, schema *config.Schema) {
	if schema == nil {
		t.Fatal("Schema is nil")
	}

	// Check root name - with new logic, single .md file determines root name
	// In createTestSchemaStructure, we have help.md with empty settings {}, so command name is "help"
	actualExpectedRoot := "help"
	if schema.Name != actualExpectedRoot {
		t.Errorf("Expected root name '%s', got '%s'", actualExpectedRoot, schema.Name)
	}

	// Debug: Print actual commands found
	t.Logf("Found %d commands:", len(schema.Commands))
	for i, cmd := range schema.Commands {
		t.Logf("  Command %d: %s (%s)", i, cmd.Name, cmd.Description)
	}

	// Should have 2 commands: git and help
	// git/git.md should now be found with the improved parsing logic
	if len(schema.Commands) != 2 {
		t.Fatalf("Expected 2 commands (help and git), got %d", len(schema.Commands))
	}

	// Find and verify each command
	var gitCmd, helpCmd *config.Command
	for _, cmd := range schema.Commands {
		switch cmd.Name {
		case "git":
			gitCmd = cmd
		case "help":
			helpCmd = cmd
		}
	}

	// Verify help command
	if helpCmd == nil {
		t.Fatal("Help command not found")
	}
	if helpCmd.Description != "Show help information." {
		t.Errorf("Help command description mismatch: %s", helpCmd.Description)
	}
	if len(helpCmd.Arguments) != 1 {
		t.Errorf("Expected 1 argument for help command, got %d", len(helpCmd.Arguments))
	}
	if len(helpCmd.Commands) != 0 {
		t.Errorf("Expected 0 subcommands for help command, got %d", len(helpCmd.Commands))
	}

	// Verify git command
	if gitCmd == nil {
		t.Fatal("Git command not found")
	}
	if gitCmd.Description != "Git version control commands." {
		t.Errorf("Git command description mismatch: %s", gitCmd.Description)
	}
	if len(gitCmd.Options) != 1 {
		t.Errorf("Expected 1 option for git command, got %d", len(gitCmd.Options))
	}
	if len(gitCmd.Commands) != 1 {
		t.Errorf("Expected 1 subcommand for git command, got %d", len(gitCmd.Commands))
	}

	// Verify git status subcommand
	if gitCmd.Commands[0].Name != "status" {
		t.Errorf("Expected git subcommand 'status', got '%s'", gitCmd.Commands[0].Name)
	}
	if gitCmd.Commands[0].Description != "Show the working tree status." {
		t.Errorf("Git status description mismatch: %s", gitCmd.Commands[0].Description)
	}
}

// Helper function to create the correct schema structure
func createCorrectSchemaStructure(baseDir string) error {
	// Create git.md in the root (this will be found)
	gitContent := `# Description

Git version control commands.

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

# Settings
` + "```json" + `
{
    "name": "git"
}
` + "```"

	err := os.WriteFile(filepath.Join(baseDir, "git.md"), []byte(gitContent), 0644)
	if err != nil {
		return err
	}

	// Create git directory for subcommands
	gitDir := filepath.Join(baseDir, "git")
	err = os.MkdirAll(gitDir, 0755)
	if err != nil {
		return err
	}

	// Create git status subcommand
	statusDir := filepath.Join(gitDir, "status")
	err = os.MkdirAll(statusDir, 0755)
	if err != nil {
		return err
	}

	statusContent := `# Description

Show the working tree status.

# Options
` + "```json" + `
[
    {
        "flags": "--short",
        "description": "Give the output in short format",
        "type": "boolean"
    }
]
` + "```" + `

# Settings
` + "```json" + `
{}
` + "```"

	err = os.WriteFile(filepath.Join(statusDir, "status.md"), []byte(statusContent), 0644)
	if err != nil {
		return err
	}

	// Create help.md in the root
	helpContent := `# Description

Show help information.

# Arguments
` + "```json" + `
[
    {
        "name": "command",
        "description": "Command to show help for",
        "type": "string",
        "default": ""
    }
]
` + "```" + `

# Settings
` + "```json" + `
{}
` + "```"

	err = os.WriteFile(filepath.Join(baseDir, "help.md"), []byte(helpContent), 0644)
	if err != nil {
		return err
	}

	return nil
}

// Helper function to verify the correct schema structure
func verifyCorrectSchema(t *testing.T, schema *config.Schema, expectedRoot string) {
	if schema == nil {
		t.Fatal("Schema is nil")
	}

	// Check root name - with new logic, multiple .md files: git.md and help.md
	// Should select first alphabetically: git.md, so root name is "git"
	actualExpectedRoot := "git"
	if schema.Name != actualExpectedRoot {
		t.Errorf("Expected root name '%s', got '%s'", actualExpectedRoot, schema.Name)
	}

	// Debug: Print actual commands found
	t.Logf("Found %d commands:", len(schema.Commands))
	for i, cmd := range schema.Commands {
		t.Logf("  Command %d: %s (%s)", i, cmd.Name, cmd.Description)
		for j, subcmd := range cmd.Commands {
			t.Logf("    Subcommand %d: %s (%s)", j, subcmd.Name, subcmd.Description)
		}
	}

	// Should have 2 commands: git and help
	if len(schema.Commands) != 2 {
		t.Fatalf("Expected 2 commands, got %d", len(schema.Commands))
	}

	// Find and verify each command
	var gitCmd, helpCmd *config.Command
	for _, cmd := range schema.Commands {
		switch cmd.Name {
		case "git":
			gitCmd = cmd
		case "help":
			helpCmd = cmd
		}
	}

	// Verify git command
	if gitCmd == nil {
		t.Fatal("Git command not found")
	}
	if gitCmd.Description != "Git version control commands." {
		t.Errorf("Git command description mismatch: %s", gitCmd.Description)
	}
	if len(gitCmd.Options) != 1 {
		t.Errorf("Expected 1 option for git command, got %d", len(gitCmd.Options))
	}
	if len(gitCmd.Commands) != 1 {
		t.Errorf("Expected 1 subcommand for git command, got %d", len(gitCmd.Commands))
	}

	// Verify git status subcommand
	if gitCmd.Commands[0].Name != "status" {
		t.Errorf("Expected git subcommand 'status', got '%s'", gitCmd.Commands[0].Name)
	}
	if gitCmd.Commands[0].Description != "Show the working tree status." {
		t.Errorf("Git status description mismatch: %s", gitCmd.Commands[0].Description)
	}

	// Verify help command
	if helpCmd == nil {
		t.Fatal("Help command not found")
	}
	if helpCmd.Description != "Show help information." {
		t.Errorf("Help command description mismatch: %s", helpCmd.Description)
	}
	if len(helpCmd.Arguments) != 1 {
		t.Errorf("Expected 1 argument for help command, got %d", len(helpCmd.Arguments))
	}
	if len(helpCmd.Commands) != 0 {
		t.Errorf("Expected 0 subcommands for help command, got %d", len(helpCmd.Commands))
	}
}
