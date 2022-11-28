package export

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"

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

func (ex *GitExporter) Name() string {
	return "git.sr.ht"
}

func (ex *GitExporter) BaseURL() string {
	return ex.baseURL
}

// A subset of gitsrht.Repository which only contains the fields we want to
// export (i.e. the ones filled in by the GraphQL query)
type GitRepoInfo struct {
	Name        string             `json:"name"`
	Description *string            `json:"description"`
	Visibility  gitsrht.Visibility `json:"visibility"`
}

func (ex *GitExporter) Export(ctx context.Context, dir string) error {
	log.Println("git.sr.ht")

	settings, err := gitsrht.SshSettings(ex.client, ctx)
	if err != nil {
		return err
	}
	sshUser := settings.Settings.SshUser

	repos, err := gitsrht.Repositories(ex.client, ctx, nil)
	if err != nil {
		return err
	}

	baseURL, err := url.Parse(ex.BaseURL())
	if err != nil {
		panic(err)
	}

	// TODO: Should we fetch & store ACLs?
	for _, repo := range repos.Results {
		repoPath := path.Join(dir, "repos", repo.Name)
		cloneURL := fmt.Sprintf("%s@%s:%s/%s", sshUser, baseURL.Host, repo.Owner.CanonicalName, repo.Name)
		if _, err := os.Stat(repoPath); err == nil {
			log.Printf("\tSkipping %s (already exists)", repo.Name)
			continue
		}

		log.Printf("\tCloning %s", repo.Name)
		cmd := exec.Command("git", "clone", "--mirror", cloneURL, repoPath)
		if err := cmd.Run(); err != nil {
			return err
		}

		repoInfo := GitRepoInfo{
			Name:        repo.Name,
			Description: repo.Description,
			Visibility:  repo.Visibility,
		}

		file, err := os.Create(path.Join(repoPath, "srht.json"))
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
