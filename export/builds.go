package export

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"git.sr.ht/~emersion/gqlclient"

	"git.sr.ht/~emersion/hut/srht/buildssrht"
)

type BuildsExporter struct {
	client  *gqlclient.Client
	http    *http.Client
	baseURL string
}

func NewBuildsExporter(client *gqlclient.Client, baseURL string, http *http.Client) *BuildsExporter {
	newHttp := *http
	newHttp.Timeout = 10 * time.Minute // XXX: Sane default?
	return &BuildsExporter{
		client:  client,
		baseURL: baseURL,
		http:    &newHttp,
	}
}

func (ex *BuildsExporter) Name() string {
	return "builds.sr.ht"
}

func (ex *BuildsExporter) BaseURL() string {
	return ex.baseURL
}

type JobInfo struct {
	Id         int32                 `json:"id"`
	Status     string                `json:"status"`
	Note       *string               `json:"note,omitempty"`
	Tags       []string              `json:"tags"`
	Visibility buildssrht.Visibility `json:"visibility"`
}

func (ex *BuildsExporter) Export(ctx context.Context, dir string) error {
	var cursor *buildssrht.Cursor
	var ret error

	for {
		jobs, err := buildssrht.ExportJobs(ex.client, ctx, cursor)
		if err != nil {
			return err
		}

		for _, job := range jobs.Results {
			if job.Status != "SUCCESS" && job.Status != "FAILED" {
				continue
			}

			base := path.Join(dir, strconv.Itoa(int(job.Id)))
			if err := os.MkdirAll(base, 0o755); err != nil {
				return err
			}

			if err := ex.exportJob(ctx, &job, base); err != nil {
				var pe partialError
				if errors.As(err, &pe) {
					ret = err
					continue
				}
				return err
			}
		}

		cursor = jobs.Cursor
		if cursor == nil {
			break
		}
	}

	return ret
}

func (ex *BuildsExporter) exportJob(ctx context.Context, job *buildssrht.Job, base string) error {
	infoPath := path.Join(base, "info.json")
	if _, err := os.Stat(infoPath); err == nil {
		log.Printf("\tSkipping #%d (already exists)", job.Id)
		return nil
	}

	log.Printf("\tJob #%d", job.Id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		job.Log.FullURL, nil)
	if err != nil {
		return err
	}
	resp, err := ex.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return partialError{fmt.Errorf("#%d: server returned non-200 status %d", job.Id, resp.StatusCode)}
	}

	file, err := os.Create(path.Join(base, "_build.log"))
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return err
	}

	var ret error
	for _, task := range job.Tasks {
		if err := ex.exportTask(ctx, ex.http, job, &task, base); err != nil {
			ret = err
		}
	}

	file, err = os.Create(infoPath)
	if err != nil {
		return err
	}
	defer file.Close()

	jobInfo := JobInfo{
		Id:         job.Id,
		Note:       job.Note,
		Tags:       job.Tags,
		Visibility: job.Visibility,
	}
	err = json.NewEncoder(file).Encode(&jobInfo)
	if err != nil {
		return err
	}

	return ret
}

func (ex *BuildsExporter) exportTask(ctx context.Context, client *http.Client, job *buildssrht.Job, task *buildssrht.Task, base string) error {
	if task.Status != "SUCCESS" && task.Status != "FAILED" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		task.Log.FullURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return partialError{fmt.Errorf("#%d: server returned non-200 status %d", job.Id, resp.StatusCode)}
	}

	file, err := os.Create(path.Join(base, fmt.Sprintf("%s.log", task.Name)))
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return err
	}

	return nil
}
