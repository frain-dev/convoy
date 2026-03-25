package postgres

import (
	log "github.com/frain-dev/convoy/pkg/logger"
)

// pkgLogger is the default package-level logger used when no logger is explicitly provided.
var pkgLogger log.Logger = log.New("postgres", log.LevelInfo)
