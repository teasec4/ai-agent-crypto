package tools

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type FileTreeTool struct{
	Root string
	MaxDepth int
}

func (t *FileTreeTool) Name() string{
	return "get_file_tree"
}

func (t *FileTreeTool) Description()string{
	return "Returns project file tree"
}

func (t *FileTreeTool)Run(params map[string]any)(string, error){
	pathParams, _ := params["path"].(string)
	if pathParams == ""{
		pathParams = "."
	}

	fullPath, err := ResolvePath(t.Root, pathParams)
	if err != nil{
		return "", err
	}

	var builder strings.Builder

	err = filepath.WalkDir(fullPath, func(path string, d fs.DirEntry, err error) error{
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(fullPath, path)
		depth := strings.Count(rel, string(os.PathSeparator))
		if depth > t.MaxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// ignore heavy dirs
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		builder.WriteString(rel + "\n")
		return nil
	} )

	return builder.String(), err
}