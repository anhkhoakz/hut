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

	"git.sr.ht/~emersion/hut/srht/pastesrht"
)

type PasteExporter struct {
	client  *gqlclient.Client
	http    *http.Client
	baseURL string
}

func NewPasteExporter(client *gqlclient.Client, baseURL string,
	http *http.Client) *PasteExporter {
	// XXX: Is this a sane default?
	newHttp := *http
	newHttp.Timeout = 10 * time.Minute
	return &PasteExporter{
		client:  client,
		http:    &newHttp,
		baseURL: baseURL,
	}
}

func (ex *PasteExporter) Name() string {
	return "paste.sr.ht"
}

func (ex *PasteExporter) BaseURL() string {
	return ex.baseURL
}

type PasteInfo struct {
	Visibility pastesrht.Visibility `json:"visibility"`
}

func (ex *PasteExporter) Export(ctx context.Context, dir string) error {
	log.Println("paste.sr.ht")

	pastes, err := pastesrht.PasteContents(ex.client, ctx)
	if err != nil {
		return err
	}

	var ret error
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

	return ret
}

func (ex *PasteExporter) exportPaste(ctx context.Context, paste *pastesrht.Paste, dir string) error {
	base := path.Join(dir, paste.Id)
	infoPath := path.Join(dir, fmt.Sprintf("%s.json", paste.Id))
	if _, err := os.Stat(infoPath); err == nil {
		log.Printf("\tSkipping %s (already exists)", paste.Id)
		return nil
	}

	log.Printf("\t%s", paste.Id)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}

	var ret error
	for _, file := range paste.Files {
		if err := ex.exportFile(ctx, paste, base, &file); err != nil {
			ret = err
		}
	}

	file, err := os.Create(infoPath)
	if err != nil {
		return err
	}
	defer file.Close()

	pasteInfo := PasteInfo{
		Visibility: paste.Visibility,
	}
	err = json.NewEncoder(file).Encode(&pasteInfo)
	if err != nil {
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
	if file.Filename != nil {
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
