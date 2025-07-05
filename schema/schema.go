package schema

import (
	"fmt"
	"strings"

	"github.com/xhd2015/cli2web/config"
)

type SchemaDir interface {
	Name() string
	ListDirs() ([]SchemaDir, error)
	ListFiles() ([]SchemaFile, error)
}

type SchemaFile interface {
	Name() string
	Read() ([]byte, error)
}

// ParseSchema converts a directory-based schema to a unified config.Schema
func ParseSchema(rootDir SchemaDir) (*config.Schema, error) {
	// Determine the root command name based on the new logic
	rootName, rootSchemaFile, err := determineCommandName(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to determine root name: %w", err)
	}
	_ = rootSchemaFile

	schema := &config.Schema{
		Name:     rootName,
		Commands: []*config.Command{},
	}

	// Parse root directory
	commands, err := parseCommands(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to parse root commands: %w", err)
	}

	schema.Commands = commands
	return schema, nil
}

// determineCommandName determines the command name based on the new logic:
// - If there is a single .md file, use that command's name
// - If multiple .md files, prefer the one matching the directory name
// - If none matches, select the first one sorted
// - If no .md files, fall back to directory name
func determineCommandName(dir SchemaDir) (string, SchemaFile, error) {
	files, err := dir.ListFiles()
	if err != nil {
		return "", nil, fmt.Errorf("failed to list files in directory: %w", err)
	}

	// Find all .md files
	var mdFiles []SchemaFile
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".md") {
			mdFiles = append(mdFiles, file)
		}
	}

	// If no .md files, fall back to directory name
	if len(mdFiles) == 0 {
		return dir.Name(), nil, nil
	}

	// If single .md file, use its command name
	if len(mdFiles) == 1 {
		cmd, err := parseCommandFromMarkdown(mdFiles[0], strings.TrimSuffix(mdFiles[0].Name(), ".md"))
		if err != nil {
			return "", nil, fmt.Errorf("failed to parse command from %s: %w", mdFiles[0].Name(), err)
		}
		return cmd.Name, mdFiles[0], nil
	}

	// Multiple .md files: prefer the one matching directory name
	dirName := dir.Name()
	for _, file := range mdFiles {
		cmdName := strings.TrimSuffix(file.Name(), ".md")
		if cmdName == dirName {
			cmd, err := parseCommandFromMarkdown(file, cmdName)
			if err != nil {
				return "", nil, fmt.Errorf("failed to parse command from %s: %w", file.Name(), err)
			}
			return cmd.Name, file, nil
		}
	}

	// No match with directory name, select first one sorted
	var fileNames []string
	for _, file := range mdFiles {
		fileNames = append(fileNames, file.Name())
	}

	// Sort the file names
	for i := 0; i < len(fileNames); i++ {
		for j := i + 1; j < len(fileNames); j++ {
			if fileNames[i] > fileNames[j] {
				fileNames[i], fileNames[j] = fileNames[j], fileNames[i]
			}
		}
	}

	// Find the first file and parse it
	firstName := fileNames[0]
	var firstFile SchemaFile
	for _, file := range mdFiles {
		if file.Name() == firstName {
			firstFile = file
			break
		}
	}

	cmd, err := parseCommandFromMarkdown(firstFile, strings.TrimSuffix(firstName, ".md"))
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse command from %s: %w", firstName, err)
	}
	return cmd.Name, firstFile, nil
}

// parseCommands recursively parses commands from a directory
func parseCommands(dir SchemaDir) ([]*config.Command, error) {
	var commands []*config.Command

	dirs, err := dir.ListDirs()
	if err != nil {
		return nil, fmt.Errorf("failed to list directories in %s: %w", dir.Name(), err)
	}

	for _, subDir := range dirs {
		cmdName, schemaFile, err := determineCommandName(subDir)
		if err != nil {
			return nil, fmt.Errorf("failed to determine command name for subdirectory %s: %w", subDir.Name(), err)
		}

		var cmd *config.Command
		// If we found a matching file, parse it as a command
		if schemaFile != nil {
			var err error
			cmd, err = parseCommandFromMarkdown(schemaFile, cmdName)
			if err != nil {
				return nil, fmt.Errorf("failed to parse command %s from subdirectory: %w", cmdName, err)
			}
		} else {
			cmd = &config.Command{
				Name: cmdName,
			}
		}

		// Recursively parse any subcommands
		subCommands, err := parseCommands(subDir)
		if err != nil {
			return nil, fmt.Errorf("failed to parse subcommands for %s: %w", cmdName, err)
		}
		cmd.Commands = subCommands

		commands = append(commands, cmd)

	}

	return commands, nil
}
