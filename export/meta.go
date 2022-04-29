package export

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"

	"git.sr.ht/~emersion/gqlclient"

	"git.sr.ht/~emersion/hut/srht/metasrht"
)

type MetaExporter struct {
	client *gqlclient.Client
}

func NewMetaExporter(client *gqlclient.Client) *MetaExporter {
	return &MetaExporter{
		client: client,
	}
}

func (ex *MetaExporter) Name() string {
	return "meta.sr.ht"
}

func (ex *MetaExporter) Export(ctx context.Context, dir string) error {
	log.Println("meta.sr.ht")
	profileFile, err := os.Create(path.Join(dir, "profile.json"))
	if err != nil {
		return err
	}
	defer profileFile.Close()

	me, err := metasrht.FetchMe(ex.client, ctx)
	if err != nil {
		return err
	}
	err = json.NewEncoder(profileFile).Encode(me)
	if err != nil {
		return err
	}

	sshFile, err := os.Create(path.Join(dir, "ssh.keys"))
	if err != nil {
		return err
	}
	defer sshFile.Close()

	user, err := metasrht.ListRawSSHKeys(ex.client, ctx)
	if err != nil {
		return err
	}
	for _, key := range user.SshKeys.Results {
		if _, err := fmt.Fprintln(sshFile, key.Key); err != nil {
			return err
		}
	}

	pgpFile, err := os.Create(path.Join(dir, "keys.pgp"))
	if err != nil {
		return err
	}
	defer pgpFile.Close()

	user, err = metasrht.ListRawPGPKeys(ex.client, ctx)
	if err != nil {
		return err
	}
	for _, key := range user.PgpKeys.Results {
		if _, err := fmt.Fprintln(pgpFile, key.Key); err != nil {
			return err
		}
	}

	return nil
}
