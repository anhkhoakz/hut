package export

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
)

const infoFilename = "info.json"

type Info struct {
	Service string `json:"service"`
	Name    string `json:"name"`
}

type Exporter interface {
	Export(ctx context.Context, dir string) error
	ExportResource(ctx context.Context, dir, owner, name string) error
	ImportResource(ctx context.Context, dir string) error
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

func readJSON(filename string, v interface{}) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewDecoder(f).Decode(v)
}

type DirResource struct {
	Info
	Path string
}

func FindDirResources(dir string) ([]DirResource, error) {
	var l []DirResource
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		f, err := os.Open(filepath.Join(path, infoFilename))
		if os.IsNotExist(err) {
			return nil
		} else if err != nil {
			return err
		}
		defer f.Close()

		var info Info
		if err := json.NewDecoder(f).Decode(&info); err != nil {
			return err
		}

		l = append(l, DirResource{
			Info: info,
			Path: path,
		})
		return filepath.SkipDir
	})
	return l, err
}
