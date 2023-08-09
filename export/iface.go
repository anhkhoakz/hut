package export

import "context"

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
