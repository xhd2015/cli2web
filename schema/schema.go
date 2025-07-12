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

// ParseOption represents an option for parsing schema
type ParseOption func(*parseConfig)

// parseConfig holds configuration for parsing
type parseConfig struct {
	maxDepth    int
	hasMaxDepth bool
}

// WithMaxDepth sets the maximum depth for parsing
func WithMaxDepth(depth int) ParseOption {
	return func(cfg *parseConfig) {
		cfg.maxDepth = depth
		cfg.hasMaxDepth = true
	}
}

// ParseSchema converts a directory-based schema to a unified config.Schema
func ParseSchema(rootDir SchemaDir, opts ...ParseOption) (*config.Schema, error) {
	cfg := &parseConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Determine the root command name based on the new logic
	rootName, rootSchemaFile, err := determineCommandName(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to determine root name: %w", err)
	}

	schema := &config.Schema{
		Name:     rootName,
		Commands: []*config.Command{},
	}

	// Parse root schema file information if available
	if rootSchemaFile != nil {
		rootCmd, err := parseCommandFromMarkdown(rootSchemaFile, rootName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse root schema file: %w", err)
		}
		// Populate schema with root command information
		schema.Description = rootCmd.Description
		schema.Examples = rootCmd.Examples
		schema.Options = rootCmd.Options
		schema.Arguments = rootCmd.Arguments
		schema.Usage = rootCmd.Usage
		schema.Notes = rootCmd.Notes
		// Override name if specified in the root command
		if rootCmd.Name != "" && rootCmd.Name != rootName {
			schema.Name = rootCmd.Name
		}
	}

	// If max depth is less than 2, don't parse any commands
	if cfg.hasMaxDepth && cfg.maxDepth < 2 {
		return schema, nil
	}

	// Parse root directory using unified function
	commands, err := parseCommandsWithConfigAndDepth(rootDir, cfg, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to parse root commands: %w", err)
	}

	schema.Commands = commands
	return schema, nil
}

// ParseSchemaWithMaxDepth converts a directory-based schema to a unified config.Schema with depth limiting
// Deprecated: Use ParseSchema with WithMaxDepth option instead
func ParseSchemaWithMaxDepth(rootDir SchemaDir, maxDepth int) (*config.Schema, error) {
	return ParseSchema(rootDir, WithMaxDepth(maxDepth))
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

// parseCommandsWithConfig recursively parses commands from a directory with configuration
func parseCommandsWithConfig(dir SchemaDir, cfg *parseConfig) ([]*config.Command, error) {
	return parseCommandsWithConfigAndDepth(dir, cfg, 0)
}

// parseCommandsFilteredWithConfig parses commands from a directory with configuration, but excludes .md files that match the directory name
func parseCommandsFilteredWithConfig(dir SchemaDir, excludeFileName string, cfg *parseConfig) ([]*config.Command, error) {
	return parseCommandsFilteredWithConfigAndDepth(dir, excludeFileName, cfg, 0)
}

// parseCommandsWithConfigAndDepth recursively parses commands from a directory with configuration and depth tracking
func parseCommandsWithConfigAndDepth(dir SchemaDir, cfg *parseConfig, currentDepth int) ([]*config.Command, error) {
	var commands []*config.Command

	// Get all files and directories
	files, err := dir.ListFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list files in %s: %w", dir.Name(), err)
	}

	dirs, err := dir.ListDirs()
	if err != nil {
		return nil, fmt.Errorf("failed to list directories in %s: %w", dir.Name(), err)
	}

	// Build maps for quick lookup
	dirNames := make(map[string]SchemaDir)
	for _, d := range dirs {
		dirNames[d.Name()] = d
	}

	mdFiles := make(map[string]SchemaFile)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".md") {
			cmdName := strings.TrimSuffix(file.Name(), ".md")
			mdFiles[cmdName] = file
		}
	}

	// Process .md files and corresponding subdirectories
	processed := make(map[string]bool)

	// First, process .md files that have corresponding subdirectories
	for cmdName, mdFile := range mdFiles {
		if subDir, hasSubdir := dirNames[cmdName]; hasSubdir {
			// Parse the command from the .md file
			cmd, err := parseCommandFromMarkdown(mdFile, cmdName)
			if err != nil {
				return nil, fmt.Errorf("failed to parse command %s from file: %w", cmdName, err)
			}

			// Add subcommands from the subdirectory (excluding the matching .md file)
			// Check depth limit if enabled
			if !cfg.hasMaxDepth || currentDepth+1 < cfg.maxDepth {
				subCommands, err := parseCommandsFilteredWithConfigAndDepth(subDir, cmdName, cfg, currentDepth+1)
				if err != nil {
					return nil, fmt.Errorf("failed to parse subcommands for %s: %w", cmdName, err)
				}
				cmd.Commands = subCommands
			}

			commands = append(commands, cmd)
			processed[cmdName] = true
		}
	}

	// Finally, process subdirectories that don't have corresponding .md files
	for _, subDir := range dirs {
		if !processed[subDir.Name()] {
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

			// Recursively parse any subcommands (excluding the matching .md file)
			// Check depth limit if enabled
			if !cfg.hasMaxDepth || currentDepth+1 < cfg.maxDepth {
				subCommands, err := parseCommandsFilteredWithConfigAndDepth(subDir, cmdName, cfg, currentDepth+1)
				if err != nil {
					return nil, fmt.Errorf("failed to parse subcommands for %s: %w", cmdName, err)
				}
				cmd.Commands = subCommands
			}

			commands = append(commands, cmd)
		}
	}

	return commands, nil
}

// parseCommandsFilteredWithConfigAndDepth parses commands from a directory with configuration and depth tracking, but excludes .md files that match the directory name
func parseCommandsFilteredWithConfigAndDepth(dir SchemaDir, excludeFileName string, cfg *parseConfig, currentDepth int) ([]*config.Command, error) {
	var commands []*config.Command

	// Get all files and directories
	files, err := dir.ListFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list files in %s: %w", dir.Name(), err)
	}

	dirs, err := dir.ListDirs()
	if err != nil {
		return nil, fmt.Errorf("failed to list directories in %s: %w", dir.Name(), err)
	}

	// Build maps for quick lookup, but exclude the specific file
	dirNames := make(map[string]SchemaDir)
	for _, d := range dirs {
		dirNames[d.Name()] = d
	}

	mdFiles := make(map[string]SchemaFile)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".md") {
			cmdName := strings.TrimSuffix(file.Name(), ".md")
			// Skip the file that matches the directory name (it's used to define the directory command)
			if cmdName == excludeFileName {
				continue
			}
			mdFiles[cmdName] = file
		}
	}

	// Process .md files and corresponding subdirectories
	processed := make(map[string]bool)

	// First, process .md files that have corresponding subdirectories
	for cmdName, mdFile := range mdFiles {
		if subDir, hasSubdir := dirNames[cmdName]; hasSubdir {
			// Parse the command from the .md file
			cmd, err := parseCommandFromMarkdown(mdFile, cmdName)
			if err != nil {
				return nil, fmt.Errorf("failed to parse command %s from file: %w", cmdName, err)
			}

			// Add subcommands from the subdirectory
			// Check depth limit if enabled
			if !cfg.hasMaxDepth || currentDepth+1 < cfg.maxDepth {
				subCommands, err := parseCommandsFilteredWithConfigAndDepth(subDir, cmdName, cfg, currentDepth+1)
				if err != nil {
					return nil, fmt.Errorf("failed to parse subcommands for %s: %w", cmdName, err)
				}
				cmd.Commands = subCommands
			}

			commands = append(commands, cmd)
			processed[cmdName] = true
		}
	}

	// Then, process standalone .md files (those without corresponding subdirectories)
	for cmdName, mdFile := range mdFiles {
		if !processed[cmdName] {
			cmd, err := parseCommandFromMarkdown(mdFile, cmdName)
			if err != nil {
				return nil, fmt.Errorf("failed to parse command %s from file: %w", cmdName, err)
			}
			commands = append(commands, cmd)
			processed[cmdName] = true
		}
	}

	// Finally, process subdirectories that don't have corresponding .md files
	for _, subDir := range dirs {
		if !processed[subDir.Name()] {
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
			// Check depth limit if enabled
			if !cfg.hasMaxDepth || currentDepth+1 < cfg.maxDepth {
				subCommands, err := parseCommandsFilteredWithConfigAndDepth(subDir, cmdName, cfg, currentDepth+1)
				if err != nil {
					return nil, fmt.Errorf("failed to parse subcommands for %s: %w", cmdName, err)
				}
				cmd.Commands = subCommands
			}

			commands = append(commands, cmd)
		}
	}

	return commands, nil
}
