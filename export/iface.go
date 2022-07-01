package export

import "context"

type Exporter interface {
	Name() string
	BaseURL() string
	Export(ctx context.Context, dir string) error
}

type partialError struct {
	error
}

func (err partialError) Unwrap() error {
	return err.error
}
