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

	"git.sr.ht/~emersion/hut/srht/pastesrht"
)

const pasteFilesDir = "files"

type PasteExporter struct {
	client *gqlclient.Client
	http   *http.Client
}

func NewPasteExporter(client *gqlclient.Client, http *http.Client) *PasteExporter {
	// XXX: Is this a sane default?
	newHttp := *http
	newHttp.Timeout = 10 * time.Minute
	return &PasteExporter{
		client: client,
		http:   &newHttp,
	}
}

type PasteInfo struct {
	Info
	Visibility pastesrht.Visibility `json:"visibility"`
}

func (ex *PasteExporter) Export(ctx context.Context, dir string) error {
	var cursor *pastesrht.Cursor
	var ret error

	for {
		pastes, err := pastesrht.PasteContents(ex.client, ctx, cursor)
		if err != nil {
			return err
		}

		for _, paste := range pastes.Results {
			if err := ex.exportPaste(ctx, &paste, dir); err != nil {
				var pe partialError
				if errors.As(err, &pe) {
					ret = err
					continue
				}
				return err
			}
		}

		cursor = pastes.Cursor
		if cursor == nil {
			break
		}
	}

	return ret
}

func (ex *PasteExporter) exportPaste(ctx context.Context, paste *pastesrht.Paste, dir string) error {
	base := path.Join(dir, paste.Id)
	infoPath := path.Join(base, infoFilename)
	if _, err := os.Stat(infoPath); err == nil {
		log.Printf("\tSkipping %s (already exists)", paste.Id)
		return nil
	}

	log.Printf("\t%s", paste.Id)
	files := path.Join(base, pasteFilesDir)
	if err := os.MkdirAll(files, 0o755); err != nil {
		return err
	}

	var ret error
	for _, file := range paste.Files {
		if err := ex.exportFile(ctx, paste, files, &file); err != nil {
			ret = err
		}
	}

	pasteInfo := PasteInfo{
		Info: Info{
			Service: "paste.sr.ht",
			Name:    paste.Id,
		},
		Visibility: paste.Visibility,
	}
	if err := writeJSON(infoPath, &pasteInfo); err != nil {
		return err
	}

	return ret
}

func (ex *PasteExporter) exportFile(ctx context.Context, paste *pastesrht.Paste, base string, file *pastesrht.File) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, string(file.Contents), nil)
	if err != nil {
		return err
	}
	resp, err := ex.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	name := paste.Id
	if file.Filename != nil && *file.Filename != "" {
		name = *file.Filename
	}

	if resp.StatusCode != http.StatusOK {
		return partialError{fmt.Errorf("%s/%s: server returned non-200 status %d", paste.Id, name, resp.StatusCode)}
	}

	f, err := os.Create(path.Join(base, name))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func (ex *PasteExporter) ImportResource(ctx context.Context, dir string) error {
	var info PasteInfo
	if err := readJSON(path.Join(dir, infoFilename), &info); err != nil {
		return err
	}

	return ex.importPaste(ctx, &info, dir)
}

func (ex *PasteExporter) importPaste(ctx context.Context, paste *PasteInfo, base string) error {
	filesPath := path.Join(base, pasteFilesDir)
	items, err := os.ReadDir(filesPath)
	if err != nil {
		return err
	}

	var files []gqlclient.Upload
	for _, item := range items {
		if item.IsDir() {
			continue
		}

		f, err := os.Open(path.Join(filesPath, item.Name()))
		if err != nil {
			return err
		}
		defer f.Close()

		var name string
		if item.Name() != paste.Name {
			name = item.Name()
		}

		files = append(files, gqlclient.Upload{
			Filename: name,
			// MIMEType is not used by the API, except for checking that it is a "text".
			// Parsing the MIME type from the extension would cause issues: ".json" is parsed as "application/json",
			// which gets rejected because it is not a "text/".
			// Since the API does not use the type besides that, always send a dummy text value.
			MIMEType: "text/plain",
			Body:     f,
		})
	}

	if _, err := pastesrht.CreatePaste(ex.client, ctx, files, paste.Visibility); err != nil {
		return fmt.Errorf("failed to create paste: %v", err)
	}
	return nil
}
