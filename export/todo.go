package export

import (
	"context"
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

const trackerFilename = "tracker.json.gz"

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
	Description *string             `json:"description"`
	Visibility  todosrht.Visibility `json:"visibility"`
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
			if err := ex.exportTracker(ctx, &tracker, base); err != nil {
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

func (ex *TodoExporter) ExportResource(ctx context.Context, dir, owner, resource string) error {
	user, err := todosrht.ExportTracker(ex.client, ctx, owner, resource)
	if err != nil {
		return err
	}
	return ex.exportTracker(ctx, user.Tracker, dir)
}

func (ex *TodoExporter) exportTracker(ctx context.Context, tracker *todosrht.Tracker, base string) error {
	infoPath := path.Join(base, infoFilename)
	if _, err := os.Stat(infoPath); err == nil {
		log.Printf("\tSkipping %s (already exists)", tracker.Name)
		return nil
	}

	dataPath := path.Join(base, trackerFilename)
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

	trackerInfo := TrackerInfo{
		Info: Info{
			Service: "todo.sr.ht",
			Name:    tracker.Name,
		},
		Description: tracker.Description,
		Visibility:  tracker.Visibility,
	}
	if err := writeJSON(infoPath, &trackerInfo); err != nil {
		return err
	}

	return nil
}

func (ex *TodoExporter) ImportResource(ctx context.Context, dir string) error {
	var info TrackerInfo
	if err := readJSON(path.Join(dir, infoFilename), &info); err != nil {
		return err
	}

	return ex.importTracker(ctx, &info, dir)
}

func (ex *TodoExporter) importTracker(ctx context.Context, tracker *TrackerInfo, base string) error {
	f, err := os.Open(path.Join(base, trackerFilename))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = todosrht.ImportTracker(ex.client, ctx, tracker.Name, tracker.Description, tracker.Visibility, gqlclient.Upload{
		Filename: trackerFilename,
		MIMEType: "application/gzip",
		Body:     f,
	})
	if err != nil {
		return fmt.Errorf("failed to import issue tracker: %v", err)
	}

	return nil
}
