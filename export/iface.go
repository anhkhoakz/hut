package export

import "context"

type Exporter interface {
	Name() string
	BaseURL() string
	Export(ctx context.Context, dir string) error
}
