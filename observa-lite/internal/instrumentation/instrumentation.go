package instrumentation

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed node.cjs
var NodeModule []byte

// Export writes a preload module without overwriting existing installations.
func Export(directory string) (string, error) {
	if directory == "" {
		return "", fmt.Errorf("informe --directory")
	}
	dir, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "vetra-node.cjs")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	if _, err = f.Write(NodeModule); err != nil {
		f.Close()
		os.Remove(path)
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	return path, nil
}
