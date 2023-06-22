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

	"git.sr.ht/~emersion/hut/srht/listssrht"
)

type ListsExporter struct {
	client  *gqlclient.Client
	http    *http.Client
	baseURL string
}

func NewListsExporter(client *gqlclient.Client, baseURL string,
	http *http.Client) *ListsExporter {
	newHttp := *http
	// XXX: Is this a sane default? Maybe large lists or slow
	// connections could require more. Would be nice to ensure a
	// constant flow of data rather than ensuring the entire request is
	// complete within a deadline. Would also be nice to use range
	// headers to be able to resume this on failure or interruption.
	newHttp.Timeout = 10 * time.Minute
	return &ListsExporter{
		client:  client,
		http:    &newHttp,
		baseURL: baseURL,
	}
}

func (ex *ListsExporter) Name() string {
	return "lists.sr.ht"
}

func (ex *ListsExporter) BaseURL() string {
	return ex.baseURL
}

// A subset of listssrht.MailingList which only contains the fields we want to
// export (i.e. the ones filled in by the GraphQL query)
type MailingListInfo struct {
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	PermitMime  []string `json:"permitMime"`
	RejectMime  []string `json:"rejectMime"`
}

func (ex *ListsExporter) Export(ctx context.Context, dir string) error {
	log.Println("lists.sr.ht")
	var cursor *listssrht.Cursor
	var ret error

	for {
		user, err := listssrht.ExportMailingLists(ex.client, ctx, cursor)
		if err != nil {
			return err
		}

		for _, list := range user.Lists.Results {
			base := path.Join(dir, list.Name)
			if err := os.MkdirAll(base, 0o755); err != nil {
				return err
			}

			if err := ex.exportList(ctx, list, base); err != nil {
				var pe partialError
				if errors.As(err, &pe) {
					ret = err
					continue
				}
				return err
			}
		}

		cursor = user.Lists.Cursor
		if cursor == nil {
			break
		}
	}

	return ret
}

func (ex *ListsExporter) exportList(ctx context.Context, list listssrht.MailingList, base string) error {
	infoPath := path.Join(base, "info.json")
	if _, err := os.Stat(infoPath); err == nil {
		log.Printf("\tSkipping %s (already exists)", list.Name)
		return nil
	}

	log.Printf("\t%s", list.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		string(list.Archive), nil)
	if err != nil {
		return err
	}
	resp, err := ex.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return partialError{fmt.Errorf("%s: server returned non-200 status %d",
			list.Name, resp.StatusCode)}
	}

	archive, err := os.Create(path.Join(base, "archive.mbox"))
	if err != nil {
		return err
	}
	defer archive.Close()
	if _, err := io.Copy(archive, resp.Body); err != nil {
		return err
	}

	file, err := os.Create(infoPath)
	if err != nil {
		return err
	}
	defer file.Close()

	listInfo := MailingListInfo{
		Name:        list.Name,
		Description: list.Description,
		PermitMime:  list.PermitMime,
		RejectMime:  list.RejectMime,
	}
	if err = json.NewEncoder(file).Encode(&listInfo); err != nil {
		return err
	}

	return nil
}
