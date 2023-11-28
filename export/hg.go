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

const hgRepositoryDir = "repository"

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
	Description   *string           `json:"description"`
	Visibility    hgsrht.Visibility `json:"visibility"`
	Readme        *string           `json:"readme"`
	NonPublishing bool              `json:"nonPublishing"`
}

func (ex *HgExporter) Export(ctx context.Context, dir string) error {
	baseURL, err := url.Parse(ex.baseURL)
	if err != nil {
		panic(err)
	}

	var cursor *hgsrht.Cursor
	for {
		repos, err := hgsrht.ExportRepositories(ex.client, ctx, cursor)
		if err != nil {
			return err
		}

		// TODO: Should we fetch & store ACLs?
		for _, repo := range repos.Results {
			repoPath := path.Join(dir, repo.Name)
			infoPath := path.Join(repoPath, infoFilename)
			clonePath := path.Join(repoPath, hgRepositoryDir)
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
				Description:   repo.Description,
				Visibility:    repo.Visibility,
				Readme:        repo.Readme,
				NonPublishing: repo.NonPublishing,
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

func (ex *HgExporter) ImportResource(ctx context.Context, dir string) error {
	baseURL, err := url.Parse(ex.baseURL)
	if err != nil {
		panic(err)
	}

	var info HgRepoInfo
	if err := readJSON(path.Join(dir, infoFilename), &info); err != nil {
		return err
	}

	description := ""
	if info.Description != nil {
		description = *info.Description
	}

	h, err := hgsrht.CreateRepository(ex.client, ctx, info.Name, info.Visibility, description)
	if err != nil {
		return fmt.Errorf("failed to create Mercurial repository: %v", err)
	}

	clonePath := path.Join(dir, hgRepositoryDir)
	cloneURL := fmt.Sprintf("ssh://hg@%s/%s/%s", baseURL.Host, h.Owner.CanonicalName, info.Name)

	cmd := exec.Command("hg", "push", "--cwd", clonePath, cloneURL)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push Mercurial repository: %v", err)
	}

	if _, err := hgsrht.UpdateRepository(ex.client, ctx, h.Id, hgsrht.RepoInput{
		Readme:        info.Readme,
		NonPublishing: &info.NonPublishing,
	}); err != nil {
		return fmt.Errorf("failed to update Mercurial repository: %v", err)
	}

	return nil
}
