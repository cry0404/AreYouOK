package main

import (
	"AreYouOK/internal/repository"
	"AreYouOK/pkg/logger"
)

func main() {
	logger.Init()
	defer logger.Sync()

	repository.RunGenerate()
}
