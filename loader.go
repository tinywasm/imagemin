//go:build !wasm

package imagemin

import (
	"bytes"
	"encoding/json"
	"os/exec"
)

func (h *Handler) listModulesReal(rootDir string) ([]string, error) {
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	cmd.Dir = rootDir
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	var dirs []string
	dec := json.NewDecoder(&out)
	for dec.More() {
		var m struct {
			Dir string
		}
		if err := dec.Decode(&m); err != nil {
			return nil, err
		}
		if m.Dir != "" {
			dirs = append(dirs, m.Dir)
		}
	}

	return dirs, nil
}

func (h *Handler) InitDefaultLoader() {
	h.listModulesFn = h.listModulesReal
}
