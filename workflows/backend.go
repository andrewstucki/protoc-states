package workflows

import (
	"github.com/microsoft/durabletask-go/backend"
	"github.com/microsoft/durabletask-go/backend/sqlite"
)

type memoryBackend struct {
	backend.Backend
}

func NewMemoryBackend(logger backend.Logger) backend.Backend {
	return &memoryBackend{
		Backend: sqlite.NewSqliteBackend(sqlite.NewSqliteOptions(""), logger),
	}
}
