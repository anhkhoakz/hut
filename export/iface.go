package export

import "context"

type Exporter interface {
	Name() string
	Export(ctx context.Context, dir string) error
}
