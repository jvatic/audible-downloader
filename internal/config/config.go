package config

import (
	"os"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
)

func Init() error {
	initLogLevel()
	return nil
}

func Dir() string {
	home, err := homedir.Dir()
	if err != nil {
		log.Fatalf("failed to load home dir: %s", err)
	}
	dir := filepath.Join(home, "audible-downloader-config")
	os.MkdirAll(dir, 0755)
	return dir
}

func initLogLevel() {
	switch os.Getenv("LOG_LEVEL") {
	case "trace":
		log.SetLevel(log.TraceLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}
