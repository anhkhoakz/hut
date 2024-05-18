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

	"git.sr.ht/~xenrox/hut/srht/hgsrht"
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
	var cursor *hgsrht.Cursor
	for {
		repos, err := hgsrht.ExportRepositories(ex.client, ctx, cursor)
		if err != nil {
			return err
		}

		for _, repo := range repos.Results {
			base := path.Join(dir, repo.Name)
			if err := ex.exportRepository(ctx, repo, base); err != nil {
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

func (ex *HgExporter) ExportResource(ctx context.Context, dir, owner, resource string) error {
	user, err := hgsrht.ExportRepository(ex.client, ctx, owner, resource)
	if err != nil {
		return err
	}
	return ex.exportRepository(ctx, user.Repository, dir)
}

func (ex *HgExporter) exportRepository(ctx context.Context, repo *hgsrht.Repository, base string) error {
	// TODO: Should we fetch & store ACLs?
	baseURL, err := url.Parse(ex.baseURL)
	if err != nil {
		panic(err)
	}
	infoPath := path.Join(base, infoFilename)
	clonePath := path.Join(base, hgRepositoryDir)
	cloneURL := fmt.Sprintf("ssh://hg@%s/%s/%s", baseURL.Host, repo.Owner.CanonicalName, repo.Name)

	if _, err := os.Stat(clonePath); err == nil {
		log.Printf("\tSkipping %s (already exists)", repo.Name)
		return nil
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}

	log.Printf("\tCloning %s", repo.Name)
	cmd := exec.CommandContext(ctx, "hg", "clone", "-U", cloneURL, clonePath)
	if err := cmd.Run(); err != nil {
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
	return writeJSON(infoPath, &repoInfo)
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
