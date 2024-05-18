package export

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"

	"git.sr.ht/~emersion/gqlclient"

	"git.sr.ht/~xenrox/hut/srht/gitsrht"
)

const gitRepositoryDir = "repository.git"

type GitExporter struct {
	client       *gqlclient.Client
	baseURL      string
	baseCloneURL string
}

func NewGitExporter(client *gqlclient.Client, baseURL string) *GitExporter {
	return &GitExporter{
		client:  client,
		baseURL: baseURL,
	}
}

// A subset of gitsrht.Repository which only contains the fields we want to
// export (i.e. the ones filled in by the GraphQL query)
type GitRepoInfo struct {
	Info
	Description *string            `json:"description"`
	Visibility  gitsrht.Visibility `json:"visibility"`
	Readme      *string            `json:"readme"`
	Head        *string            `json:"head"`
}

func (ex *GitExporter) Export(ctx context.Context, dir string) error {
	var cursor *gitsrht.Cursor
	for {
		repos, err := gitsrht.ExportRepositories(ex.client, ctx, cursor)
		if err != nil {
			return err
		}

		for _, repo := range repos.Results {
			base := path.Join(dir, repo.Name)
			if err := ex.exportRepository(ctx, &repo, base); err != nil {
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

func (ex *GitExporter) ExportResource(ctx context.Context, dir, owner, resource string) error {
	user, err := gitsrht.ExportRepository(ex.client, ctx, owner, resource)
	if err != nil {
		return err
	}
	return ex.exportRepository(ctx, user.Repository, dir)
}

func (ex *GitExporter) exportRepository(ctx context.Context, repo *gitsrht.Repository, base string) error {
	// Cache base clone URL in exporter.
	if ex.baseCloneURL == "" {
		settings, err := gitsrht.SshSettings(ex.client, ctx)
		if err != nil {
			return err
		}
		sshUser := settings.Settings.SshUser

		baseURL, err := url.Parse(ex.baseURL)
		if err != nil {
			panic(err)
		}
		ex.baseCloneURL = fmt.Sprintf("%s@%s", sshUser, baseURL.Host)
	}

	// TODO: Should we fetch & store ACLs?
	infoPath := path.Join(base, infoFilename)
	clonePath := path.Join(base, gitRepositoryDir)
	cloneURL := fmt.Sprintf("%s:%s/%s", ex.baseCloneURL, repo.Owner.CanonicalName, repo.Name)

	if _, err := os.Stat(clonePath); err == nil {
		log.Printf("\tSkipping %s (already exists)", repo.Name)
		return nil
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}

	log.Printf("\tCloning %s", repo.Name)
	cmd := exec.CommandContext(ctx, "git", "clone", "--mirror", cloneURL, clonePath)
	if err := cmd.Run(); err != nil {
		return err
	}

	var head *string
	if repo.HEAD != nil {
		h := strings.TrimPrefix(repo.HEAD.Name, "refs/heads/")
		head = &h
	}

	repoInfo := GitRepoInfo{
		Info: Info{
			Service: "git.sr.ht",
			Name:    repo.Name,
		},
		Description: repo.Description,
		Visibility:  repo.Visibility,
		Readme:      repo.Readme,
		Head:        head,
	}
	return writeJSON(infoPath, &repoInfo)
}

func (ex *GitExporter) ImportResource(ctx context.Context, dir string) error {
	settings, err := gitsrht.SshSettings(ex.client, ctx)
	if err != nil {
		return fmt.Errorf("failed to get Git SSH settings: %v", err)
	}
	sshUser := settings.Settings.SshUser

	baseURL, err := url.Parse(ex.baseURL)
	if err != nil {
		panic(err)
	}

	var info GitRepoInfo
	if err := readJSON(path.Join(dir, infoFilename), &info); err != nil {
		return err
	}

	g, err := gitsrht.CreateRepository(ex.client, ctx, info.Name, info.Visibility, info.Description, nil)
	if err != nil {
		return fmt.Errorf("failed to create Git repository: %v", err)
	}

	clonePath := path.Join(dir, gitRepositoryDir)
	cloneURL := fmt.Sprintf("%s@%s:%s/%s", sshUser, baseURL.Host, g.Owner.CanonicalName, info.Name)

	cmd := exec.Command("git", "-C", clonePath, "push", "--mirror", cloneURL)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push Git repository: %v", err)
	}

	if _, err := gitsrht.UpdateRepository(ex.client, ctx, g.Id, gitsrht.RepoInput{
		Readme: info.Readme,
		HEAD:   info.Head,
	}); err != nil {
		return fmt.Errorf("failed to update Git repository: %v", err)
	}

	return nil
}
