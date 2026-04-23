package imagemin

import (
	"path/filepath"
	"sync"
	"time"
)

type Handler struct {
	mu            sync.Mutex
	config        *Config
	log           func(messages ...any)
	listModulesFn func(rootDir string) ([]string, error)
}

type Config struct {
	RootDir   string
	OutputDir string
	Quality   int
}

func New(c *Config) *Handler {
	return &Handler{
		config: c,
		log:    func(messages ...any) {},
	}
}

func (h *Handler) SetLog(fn func(messages ...any)) {
	h.log = fn
}

func (h *Handler) SetListModulesFn(fn func(rootDir string) ([]string, error)) {
	h.listModulesFn = fn
}

func (h *Handler) Name() string { return "imagemin" }

func (h *Handler) SupportedExtensions() []string { return []string{} }

func (h *Handler) NewFileEvent(fileName, extension, filePath, event string) error { return nil }

func (h *Handler) UnobservedFiles() []string {
	return []string{h.config.OutputDir, filepath.Join(h.config.OutputDir, ".cache.json")}
}

func (h *Handler) MainInputFileRelativePath() string { return "" }

// WaitForLoad waits for the initial load to complete.
func (h *Handler) WaitForLoad(timeout time.Duration) {
	// Currently LoadImages is synchronous, so this is just for interface compatibility.
}
