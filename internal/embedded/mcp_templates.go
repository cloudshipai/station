package embedded

import (
	"embed"
	"io/fs"
)

//go:embed mcp-servers/*.json
var mcpTemplatesFS embed.FS

func GetMCPTemplatesFS() (fs.FS, error) {
	return fs.Sub(mcpTemplatesFS, "mcp-servers")
}

func GetMCPTemplateFiles() ([]string, error) {
	subFS, err := GetMCPTemplatesFS()
	if err != nil {
		return nil, err
	}

	var files []string
	err = fs.WalkDir(subFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && len(path) > 5 && path[len(path)-5:] == ".json" {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

func ReadMCPTemplate(filename string) ([]byte, error) {
	subFS, err := GetMCPTemplatesFS()
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(subFS, filename)
}
