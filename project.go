package main

import (
	"errors"
	"os"
	"path/filepath"

	"codeberg.org/emersion/go-scfg"
)

type projectConfig struct {
	Tracker     string `scfg:"tracker"`
	DevList     string `scfg:"development-mailing-list"`
	PatchPrefix bool   `scfg:"patch-prefix"`
}

func loadProjectConfig() (*projectConfig, error) {
	fileName, err := findProjectConfig()
	if err != nil {
		return nil, err
	}

	if fileName == "" {
		return nil, nil
	}

	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := new(projectConfig)
	if err := scfg.NewDecoder(f).Decode(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func findProjectConfig() (string, error) {
	cur, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		fileName := filepath.Join(cur, ".hut.scfg")
		_, err := os.Stat(fileName)
		if err == nil {
			return fileName, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		next := filepath.Dir(cur)
		if next == cur {
			break
		}
		cur = next
	}

	return "", nil
}
