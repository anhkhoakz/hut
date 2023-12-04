package export

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"git.sr.ht/~emersion/gqlclient"

	"git.sr.ht/~emersion/hut/srht/metasrht"
)

const (
	sshKeysFilename = "ssh.keys"
	pgpKeysFilename = "keys.pgp"
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

	sshFile, err := os.Create(path.Join(dir, sshKeysFilename))
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

	pgpFile, err := os.Create(path.Join(dir, pgpKeysFilename))
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

	if err := writeJSON(path.Join(dir, infoFilename), &Info{
		Service: "meta.sr.ht",
		Name:    me.CanonicalName,
	}); err != nil {
		return err
	}

	return nil
}

func (ex *MetaExporter) ExportResource(ctx context.Context, dir, owner, resource string) error {
	return fmt.Errorf("exporting individual meta resources is not supported")
}

func (ex *MetaExporter) ImportResource(ctx context.Context, dir string) error {
	sshFile, err := os.Open(path.Join(dir, sshKeysFilename))
	if err != nil {
		return err
	}
	defer sshFile.Close()

	sshScanner := bufio.NewScanner(sshFile)
	for sshScanner.Scan() {
		if sshScanner.Text() == "" {
			continue
		}
		if _, err := metasrht.CreateSSHKey(ex.client, ctx, sshScanner.Text()); err != nil {
			log.Printf("Error importing SSH key: %v", err)
			continue
		}
	}
	if sshScanner.Err() != nil {
		return err
	}

	pgpFile, err := os.Open(path.Join(dir, pgpKeysFilename))
	if err != nil {
		return err
	}
	defer pgpFile.Close()

	var key strings.Builder
	pgpScanner := bufio.NewScanner(pgpFile)
	for pgpScanner.Scan() {
		if strings.HasPrefix(pgpScanner.Text(), "-----BEGIN") {
			key.Reset()
		}
		key.WriteString(pgpScanner.Text())
		key.WriteByte('\n')
		if strings.HasPrefix(pgpScanner.Text(), "-----END") {
			if _, err := metasrht.CreatePGPKey(ex.client, ctx, key.String()); err != nil {
				log.Printf("Error importing PGP key: %v", err)
				continue
			}
			key.Reset()
		}
	}
	if pgpScanner.Err() != nil {
		return err
	}
	if strings.TrimSpace(key.String()) != "" {
		log.Printf("Error importing PGP key: malformed file")
	}

	return nil
}
