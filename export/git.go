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

	"git.sr.ht/~emersion/hut/srht/gitsrht"
)

type GitExporter struct {
	client  *gqlclient.Client
	baseURL string
}

func NewGitExporter(client *gqlclient.Client, baseURL string) *GitExporter {
	return &GitExporter{client, baseURL}
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
	settings, err := gitsrht.SshSettings(ex.client, ctx)
	if err != nil {
		return err
	}
	sshUser := settings.Settings.SshUser

	baseURL, err := url.Parse(ex.baseURL)
	if err != nil {
		panic(err)
	}

	var cursor *gitsrht.Cursor
	for {
		repos, err := gitsrht.ExportRepositories(ex.client, ctx, cursor)
		if err != nil {
			return err
		}

		// TODO: Should we fetch & store ACLs?
		for _, repo := range repos.Results {
			repoPath := path.Join(dir, repo.Name)
			infoPath := path.Join(repoPath, infoFilename)
			clonePath := path.Join(repoPath, "repository.git")
			cloneURL := fmt.Sprintf("%s@%s:%s/%s", sshUser, baseURL.Host, repo.Owner.CanonicalName, repo.Name)

			if _, err := os.Stat(clonePath); err == nil {
				log.Printf("\tSkipping %s (already exists)", repo.Name)
				continue
			}
			if err := os.MkdirAll(repoPath, 0o755); err != nil {
				return err
			}

			log.Printf("\tCloning %s", repo.Name)
			cmd := exec.Command("git", "clone", "--mirror", cloneURL, clonePath)
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
