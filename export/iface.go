package export

import (
	"context"
	"encoding/json"
	"os"
)

const infoFilename = "info.json"

type Info struct {
	Service string `json:"service"`
	Name    string `json:"name"`
}

type Exporter interface {
	Export(ctx context.Context, dir string) error
}

type partialError struct {
	error
}

func (err partialError) Unwrap() error {
	return err.error
}

func writeJSON(filename string, v interface{}) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(v); err != nil {
		return err
	}

	return f.Close()
}
