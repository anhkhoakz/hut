package export

import (
	"context"
	"fmt"
	"os"
	"path"

	"git.sr.ht/~emersion/gqlclient"

	"git.sr.ht/~emersion/hut/srht/metasrht"
)

type MetaExporter struct {
	client *gqlclient.Client
}

func NewMetaExporter(client *gqlclient.Client) *MetaExporter {
	return &MetaExporter{client}
}

func (ex *MetaExporter) Export(ctx context.Context, dir string) error {
	me, err := metasrht.FetchMe(ex.client, ctx)
	if err != nil {
		return err
	}
	if err := writeJSON(path.Join(dir, "profile.json"), me); err != nil {
		return err
	}

	var cursor *metasrht.Cursor

	sshFile, err := os.Create(path.Join(dir, "ssh.keys"))
	if err != nil {
		return err
	}
	defer sshFile.Close()

	for {
		user, err := metasrht.ListRawSSHKeys(ex.client, ctx, cursor)
		if err != nil {
			return err
		}

		for _, key := range user.SshKeys.Results {
			if _, err := fmt.Fprintln(sshFile, key.Key); err != nil {
				return err
			}
		}

		cursor = user.SshKeys.Cursor
		if cursor == nil {
			break
		}
	}

	pgpFile, err := os.Create(path.Join(dir, "keys.pgp"))
	if err != nil {
		return err
	}
	defer pgpFile.Close()

	for {
		user, err := metasrht.ListRawPGPKeys(ex.client, ctx, cursor)
		if err != nil {
			return err
		}

		for _, key := range user.PgpKeys.Results {
			if _, err := fmt.Fprintln(pgpFile, key.Key); err != nil {
				return err
			}
		}

		cursor = user.PgpKeys.Cursor
		if cursor == nil {
			break
		}
	}

	return nil
}
