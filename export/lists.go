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

	"git.sr.ht/~emersion/hut/srht/listssrht"
)

const archiveFilename = "archive.mbox"

type ListsExporter struct {
	client *gqlclient.Client
	http   *http.Client
}

func NewListsExporter(client *gqlclient.Client, http *http.Client) *ListsExporter {
	newHttp := *http
	// XXX: Is this a sane default? Maybe large lists or slow
	// connections could require more. Would be nice to ensure a
	// constant flow of data rather than ensuring the entire request is
	// complete within a deadline. Would also be nice to use range
	// headers to be able to resume this on failure or interruption.
	newHttp.Timeout = 10 * time.Minute
	return &ListsExporter{
		client: client,
		http:   &newHttp,
	}
}

// A subset of listssrht.MailingList which only contains the fields we want to
// export (i.e. the ones filled in by the GraphQL query)
type MailingListInfo struct {
	Info
	Description *string              `json:"description"`
	Visibility  listssrht.Visibility `json:"visibility"`
	PermitMime  []string             `json:"permitMime"`
	RejectMime  []string             `json:"rejectMime"`
}

func (ex *ListsExporter) Export(ctx context.Context, dir string) error {
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
	infoPath := path.Join(base, infoFilename)
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

	archive, err := os.Create(path.Join(base, archiveFilename))
	if err != nil {
		return err
	}
	defer archive.Close()
	if _, err := io.Copy(archive, resp.Body); err != nil {
		return err
	}

	listInfo := MailingListInfo{
		Info: Info{
			Service: "lists.sr.ht",
			Name:    list.Name,
		},
		Description: list.Description,
		Visibility:  list.Visibility,
		PermitMime:  list.PermitMime,
		RejectMime:  list.RejectMime,
	}
	if err := writeJSON(infoPath, &listInfo); err != nil {
		return err
	}

	return nil
}

func (ex *ListsExporter) ImportResource(ctx context.Context, dir string) error {
	var info MailingListInfo
	if err := readJSON(path.Join(dir, infoFilename), &info); err != nil {
		return err
	}

	return ex.importList(ctx, &info, dir)
}

func (ex *ListsExporter) importList(ctx context.Context, list *MailingListInfo, base string) error {
	l, err := listssrht.CreateMailingList(ex.client, ctx, list.Name, list.Description, list.Visibility)
	if err != nil {
		return fmt.Errorf("failed to create mailing list: %v", err)
	}

	if _, err := listssrht.UpdateMailingList(ex.client, ctx, l.Id, listssrht.MailingListInput{
		PermitMime: list.PermitMime,
		RejectMime: list.RejectMime,
	}); err != nil {
		return fmt.Errorf("failed to update mailing list: %v", err)
	}

	archive, err := os.Open(path.Join(base, archiveFilename))
	if err != nil {
		return err
	}
	defer archive.Close()

	if _, err := listssrht.ImportMailingListSpool(ex.client, ctx, l.Id, gqlclient.Upload{
		Filename: archiveFilename,
		MIMEType: "application/mbox",
		Body:     archive,
	}); err != nil {
		return fmt.Errorf("failed to import mailing list emails: %v", err)
	}

	return nil
}
