package export

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"

	"git.sr.ht/~emersion/gqlclient"

	"git.sr.ht/~emersion/hut/srht/hgsrht"
)

type HgExporter struct {
	client  *gqlclient.Client
	baseURL string
}

func NewHgExporter(client *gqlclient.Client, baseURL string) *HgExporter {
	return &HgExporter{client, baseURL}
}

// A subset of hgsrht.Repository which only contains the fields we want to
// export (i.e. the ones filled in by the GraphQL query)
type HgRepoInfo struct {
	Info
	Description *string           `json:"description"`
	Visibility  hgsrht.Visibility `json:"visibility"`
}

func (ex *HgExporter) Export(ctx context.Context, dir string) error {
	baseURL, err := url.Parse(ex.baseURL)
	if err != nil {
		panic(err)
	}

	var cursor *hgsrht.Cursor
	for {
		repos, err := hgsrht.Repositories(ex.client, ctx, cursor)
		if err != nil {
			return err
		}

		// TODO: Should we fetch & store ACLs?
		for _, repo := range repos.Results {
			repoPath := path.Join(dir, repo.Name)
			infoPath := path.Join(repoPath, infoFilename)
			clonePath := path.Join(repoPath, "repository.git")
			cloneURL := fmt.Sprintf("ssh://hg@%s/%s/%s", baseURL.Host, repo.Owner.CanonicalName, repo.Name)

			if _, err := os.Stat(clonePath); err == nil {
				log.Printf("\tSkipping %s (already exists)", repo.Name)
				continue
			}
			if err := os.MkdirAll(repoPath, 0o755); err != nil {
				return err
			}

			log.Printf("\tCloning %s", repo.Name)
			cmd := exec.Command("hg", "clone", "-U", cloneURL, clonePath)
			err := cmd.Run()
			if err != nil {
				return err
			}

			repoInfo := HgRepoInfo{
				Info: Info{
					Service: "hg.sr.ht",
					Name:    repo.Name,
				},
				Description: repo.Description,
				Visibility:  repo.Visibility,
			}
			if err := writeJSON(infoPath, &repoInfo); err != nil {
				return err
			}
		}

		cursor = repos.Cursor
		if cursor == nil {
			break
		}
	}

	return nil
}
