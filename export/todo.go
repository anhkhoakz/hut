package export

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"git.sr.ht/~emersion/gqlclient"

	"git.sr.ht/~emersion/hut/srht/todosrht"
)

type TodoExporter struct {
	client *gqlclient.Client
	http   *http.Client
}

func NewTodoExporter(client *gqlclient.Client, http *http.Client) *TodoExporter {
	newHttp := *http
	// XXX: Is this a sane default?
	newHttp.Timeout = 10 * time.Minute
	return &TodoExporter{
		client: client,
		http:   &newHttp,
	}
}

type TrackerInfo struct {
	Info
}

func (ex *TodoExporter) Export(ctx context.Context, dir string) error {
	var cursor *todosrht.Cursor
	var ret error

	for {
		trackers, err := todosrht.ExportTrackers(ex.client, ctx, cursor)
		if err != nil {
			return err
		}

		for _, tracker := range trackers.Results {
			base := path.Join(dir, tracker.Name)

			if err := ex.exportTracker(ctx, tracker, base); err != nil {
				var pe partialError
				if errors.As(err, &pe) {
					ret = err
					continue
				}
				return err
			}
		}

		cursor = trackers.Cursor
		if cursor == nil {
			break
		}
	}

	return ret
}

func (ex *TodoExporter) exportTracker(ctx context.Context, tracker todosrht.Tracker, base string) error {
	infoPath := path.Join(base, infoFilename)
	dataPath := path.Join(base, "tracker.json.gz")
	log.Printf("\t%s", tracker.Name)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, string(tracker.Export), nil)
	if err != nil {
		return err
	}
	resp, err := ex.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return partialError{fmt.Errorf("%s: server returned non-200 status %d", tracker.Name, resp.StatusCode)}
	}

	f, err := os.Create(dataPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}

	file, err := os.Create(infoPath)
	if err != nil {
		return err
	}
	defer file.Close()

	trackerInfo := PasteInfo{
		Info: Info{
			Service: "todo.sr.ht",
			Name:    tracker.Name,
		},
	}
	err = json.NewEncoder(file).Encode(&trackerInfo)
	if err != nil {
		return err
	}

	return nil
}
