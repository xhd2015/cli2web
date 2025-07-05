package schema

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/xhd2015/cli2web/config"
)

// ParseSchemaFromDir parses a schema from a filesystem directory
func ParseSchemaFromDir(dirPath string) (*config.Schema, error) {
	return ParseSchema(NewFSSchemaDir(dirPath))
}

// ParseSchemaFromEmbed parses a schema from an embedded filesystem
func ParseSchemaFromEmbed(fs embed.FS, path string) (*config.Schema, error) {
	return ParseSchema(NewEmbedSchemaDir(fs, path))
}

// ParseSchemaFromFS parses a schema from any fs.FS filesystem
func ParseSchemaFromFS(filesystem fs.FS, path string) (*config.Schema, error) {
	return ParseSchema(NewGenericFSSchemaDir(filesystem, path))
}

// FSSchemaDir implements SchemaDir for filesystem directories
type FSSchemaDir struct {
	path string
}

// NewFSSchemaDir creates a new filesystem-based schema directory
func NewFSSchemaDir(path string) *FSSchemaDir {
	return &FSSchemaDir{path: path}
}

func (d *FSSchemaDir) Name() string {
	return filepath.Base(d.path)
}

func (d *FSSchemaDir) ListDirs() ([]SchemaDir, error) {
	entries, err := os.ReadDir(d.path)
	if err != nil {
		return nil, err
	}

	var dirs []SchemaDir
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, &FSSchemaDir{path: filepath.Join(d.path, entry.Name())})
		}
	}
	return dirs, nil
}

func (d *FSSchemaDir) ListFiles() ([]SchemaFile, error) {
	entries, err := os.ReadDir(d.path)
	if err != nil {
		return nil, err
	}

	var files []SchemaFile
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, &FSSchemaFile{path: filepath.Join(d.path, entry.Name())})
		}
	}
	return files, nil
}

// FSSchemaFile implements SchemaFile for filesystem files
type FSSchemaFile struct {
	path string
}

func (f *FSSchemaFile) Name() string {
	return filepath.Base(f.path)
}

func (f *FSSchemaFile) Read() ([]byte, error) {
	return os.ReadFile(f.path)
}

// EmbedSchemaDir implements SchemaDir for embedded filesystem directories
type EmbedSchemaDir struct {
	fs   embed.FS
	path string
}

// NewEmbedSchemaDir creates a new embedded filesystem-based schema directory
func NewEmbedSchemaDir(fs embed.FS, path string) *EmbedSchemaDir {
	return &EmbedSchemaDir{fs: fs, path: path}
}

func (d *EmbedSchemaDir) Name() string {
	if d.path == "." || d.path == "" {
		return "root"
	}
	return filepath.Base(d.path)
}

func (d *EmbedSchemaDir) ListDirs() ([]SchemaDir, error) {
	entries, err := d.fs.ReadDir(d.path)
	if err != nil {
		return nil, err
	}

	var dirs []SchemaDir
	for _, entry := range entries {
		if entry.IsDir() {
			subPath := filepath.Join(d.path, entry.Name())
			dirs = append(dirs, &EmbedSchemaDir{fs: d.fs, path: subPath})
		}
	}
	return dirs, nil
}

func (d *EmbedSchemaDir) ListFiles() ([]SchemaFile, error) {
	entries, err := d.fs.ReadDir(d.path)
	if err != nil {
		return nil, err
	}

	var files []SchemaFile
	for _, entry := range entries {
		if !entry.IsDir() {
			filePath := filepath.Join(d.path, entry.Name())
			files = append(files, &EmbedSchemaFile{fs: d.fs, path: filePath})
		}
	}
	return files, nil
}

// EmbedSchemaFile implements SchemaFile for embedded filesystem files
type EmbedSchemaFile struct {
	fs   embed.FS
	path string
}

func (f *EmbedSchemaFile) Name() string {
	return filepath.Base(f.path)
}

func (f *EmbedSchemaFile) Read() ([]byte, error) {
	return f.fs.ReadFile(f.path)
}

// GenericFSSchemaDir implements SchemaDir for any fs.FS filesystem
type GenericFSSchemaDir struct {
	fs   fs.FS
	path string
}

// NewGenericFSSchemaDir creates a new generic filesystem-based schema directory
func NewGenericFSSchemaDir(filesystem fs.FS, path string) *GenericFSSchemaDir {
	return &GenericFSSchemaDir{fs: filesystem, path: path}
}

func (d *GenericFSSchemaDir) Name() string {
	if d.path == "." || d.path == "" {
		return "root"
	}
	return filepath.Base(d.path)
}

func (d *GenericFSSchemaDir) ListDirs() ([]SchemaDir, error) {
	entries, err := fs.ReadDir(d.fs, d.path)
	if err != nil {
		return nil, err
	}

	var dirs []SchemaDir
	for _, entry := range entries {
		if entry.IsDir() {
			subPath := filepath.Join(d.path, entry.Name())
			dirs = append(dirs, &GenericFSSchemaDir{fs: d.fs, path: subPath})
		}
	}
	return dirs, nil
}

func (d *GenericFSSchemaDir) ListFiles() ([]SchemaFile, error) {
	entries, err := fs.ReadDir(d.fs, d.path)
	if err != nil {
		return nil, err
	}

	var files []SchemaFile
	for _, entry := range entries {
		if !entry.IsDir() {
			filePath := filepath.Join(d.path, entry.Name())
			files = append(files, &GenericFSSchemaFile{fs: d.fs, path: filePath})
		}
	}
	return files, nil
}

// GenericFSSchemaFile implements SchemaFile for any fs.FS filesystem
type GenericFSSchemaFile struct {
	fs   fs.FS
	path string
}

func (f *GenericFSSchemaFile) Name() string {
	return filepath.Base(f.path)
}

func (f *GenericFSSchemaFile) Read() ([]byte, error) {
	return fs.ReadFile(f.fs, f.path)
}
