package export

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

func (ex *HgExporter) Name() string {
	return "hg.sr.ht"
}

func (ex *HgExporter) BaseURL() string {
	return ex.baseURL
}

// A subset of hgsrht.Repository which only contains the fields we want to
// export (i.e. the ones filled in by the GraphQL query)
type HgRepoInfo struct {
	Name        string            `json:"name"`
	Description *string           `json:"description"`
	Visibility  hgsrht.Visibility `json:"visibility"`
}

func (ex *HgExporter) Export(ctx context.Context, dir string) error {
	log.Println("hg.sr.ht")

	repos, err := hgsrht.Repositories(ex.client, ctx)
	if err != nil {
		return err
	}

	// TODO: Should we fetch & store ACLs?
	for _, repo := range repos.Results {
		repoPath := path.Join(dir, "repos", repo.Name)
		// XXX: May want to use SSH so we can clone private repositories, but we
		// probably want to advise the user to set up an SSH agent
		cloneURL := fmt.Sprintf("%s/%s/%s", ex.BaseURL(),
			repo.Owner.CanonicalName, repo.Name)
		if _, err := os.Stat(repoPath); err == nil {
			log.Printf("\tSkipping %s (already exists)", repo.Name)
			continue
		}

		log.Printf("\tCloning %s", repo.Name)
		cmd := exec.Command("hg", "clone", "-U", cloneURL, repoPath)
		err := cmd.Run()
		if err != nil {
			return err
		}

		repoInfo := HgRepoInfo{
			Name:        repo.Name,
			Description: repo.Description,
			Visibility:  repo.Visibility,
		}

		file, err := os.Create(path.Join(repoPath, ".hg", "srht.json"))
		if err != nil {
			return err
		}
		err = json.NewEncoder(file).Encode(&repoInfo)
		file.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
