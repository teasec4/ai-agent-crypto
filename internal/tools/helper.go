package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

func ResolvePath(root, input string) (string, error){
	// getting root path
	clean := filepath.Clean("/" + input)
	full := filepath.Join(root, clean)

	if !strings.HasPrefix(full, root) {

		return "", fmt.Errorf("access denied")
	}
	return full, nil
}