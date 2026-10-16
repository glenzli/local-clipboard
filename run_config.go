package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type runConfig struct {
	Server string `json:"server"`
	Name   string `json:"name"`
	token  string
}

func startRunLog(path string) error {
	// A background client has no console; keep diagnostics beside its settings.
	output, err := os.OpenFile(filepath.Join(filepath.Dir(path), "local-clipboard.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open client log: %w", err)
	}
	log.SetOutput(output)
	os.Stdout, os.Stderr = output, output
	return nil
}

func applyRunConfig(path string, fs *flag.FlagSet, server, name, token *string) error {
	cfg, err := loadRunConfig(path)
	if err != nil {
		return err
	}
	explicit := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	if !explicit["server"] {
		*server = cfg.Server
	}
	if !explicit["name"] && cfg.Name != "" {
		*name = cfg.Name
	}
	if !explicit["token"] {
		*token = cfg.token
	}
	return nil
}
