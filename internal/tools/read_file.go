package tools

import (
	"fmt"
	"os"
	"unicode/utf8"
)

type ReadFile struct{
	Root     string
	MaxBytes int
}

func (r *ReadFile) Name() string{
	return "read_file"
}

func(r *ReadFile) Description() string{
	return "Reads file content"
}

func (r *ReadFile)Run(params map[string]any)(string, error){
	pathArg, ok := params["path"].(string)
	if !ok {
		return "", fmt.Errorf("path required")
	}

	fullPath, err := ResolvePath(r.Root, pathArg)
	if err != nil {
		return "", err
	}
	
	data, err := os.ReadFile(fullPath)
	if err != nil{
		return "", err
	}


	if len(data) > r.MaxBytes {

		data = data[:r.MaxBytes]
	}
	// простая проверка на бинарник
	if !utf8.Valid(data) {
		return "", fmt.Errorf("binary file not supported")
	}
	
	return string(data), nil
}