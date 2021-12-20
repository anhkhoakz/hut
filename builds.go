package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"git.sr.ht/~emersion/hut/srht/buildssrht"
)

func newBuildsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "builds",
		Short: "Use the builds API",
	}
	cmd.AddCommand(newBuildsSubmitCommand())
	return cmd
}

func newBuildsSubmitCommand() *cobra.Command {
	var follow bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("builds")

		filenames := args
		if len(args) == 0 {
			if _, err := os.Stat(".build.yml"); err == nil {
				filenames = append(filenames, ".build.yml")
			}
			if matches, err := filepath.Glob(".build/*.yml"); err == nil {
				filenames = append(filenames, matches...)
			}
		}

		if len(filenames) == 0 {
			log.Fatal("no build manifest found")
		}
		if len(filenames) > 1 && follow {
			log.Fatal("--follow cannot be used when submitting multiple jobs")
		}

		for _, name := range filenames {
			var b []byte
			var err error
			if name == "-" {
				b, err = io.ReadAll(os.Stdin)
			} else {
				b, err = os.ReadFile(name)
			}
			if err != nil {
				log.Fatalf("failed to read manifest from %q: %v", name, err)
			}

			job, err := buildssrht.Submit(c.Client, ctx, string(b))
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("%v/%v/job/%v\n", c.BaseURL, job.Owner.CanonicalName, job.Id)

			if follow {
				job, err := c.followJob(context.Background(), job.Id)
				if err != nil {
					log.Fatal(err)
				}
				if job.Status != buildssrht.JobStatusSuccess {
					os.Exit(1)
				}
			}
		}
	}

	cmd := &cobra.Command{
		Use:   "submit [manifest...]",
		Short: "Submit a build manifest",
		Run:   run,
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow build logs")
	return cmd
}

type buildLog struct {
	offset int64
	done   bool
}

func (c *Client) followJob(ctx context.Context, id int32) (*buildssrht.Job, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	logs := make(map[string]*buildLog)

	for {
		// TODO: rig up timeout to context
		job, err := buildssrht.Monitor(c.Client, ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to monitor job: %v", err)
		}

		if len(logs) == 0 {
			logs[""] = new(buildLog)
			for _, task := range job.Tasks {
				logs[task.Name] = new(buildLog)
			}
		}

		if err := fetchJobLogs(ctx, logs[""], job); err != nil {
			return nil, fmt.Errorf("failed to fetch job logs: %v", err)
		}
		for _, task := range job.Tasks {
			if err := fetchTaskLogs(ctx, logs[task.Name], task); err != nil {
				return nil, fmt.Errorf("failed to fetch task %q logs: %v", task.Name, err)
			}
		}

		if jobStatusDone(job.Status) {
			fmt.Printf("%v %v\n", jobStatusIcon(job.Status), job.Status)
			return job, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			// Continue looping
		}
	}
}

func fetchJobLogs(ctx context.Context, l *buildLog, job *buildssrht.Job) error {
	switch job.Status {
	case buildssrht.JobStatusPending, buildssrht.JobStatusQueued:
		return nil
	}

	if err := fetchBuildLogs(ctx, l, job.Log.FullURL); err != nil {
		return err
	}

	l.done = jobStatusDone(job.Status)
	return nil
}

func fetchTaskLogs(ctx context.Context, l *buildLog, task *buildssrht.Task) error {
	switch task.Status {
	case buildssrht.TaskStatusPending:
		return nil
	}

	if err := fetchBuildLogs(ctx, l, task.Log.FullURL); err != nil {
		return err
	}

	switch task.Status {
	case buildssrht.TaskStatusPending, buildssrht.TaskStatusRunning:
		return nil
	}

	l.done = true
	return nil
}

func fetchBuildLogs(ctx context.Context, l *buildLog, url string) error {
	if l.done {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%v-", l.offset))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("invalid HTTP status: want Partial Content, got: %v %v", resp.StatusCode, resp.Status)
	}

	var rangeStart, rangeEnd int64
	var rangeSize string
	_, err = fmt.Sscanf(resp.Header.Get("Content-Range"), "bytes %d-%d/%s", &rangeStart, &rangeEnd, &rangeSize)
	if err != nil {
		return fmt.Errorf("failed to parse Content-Range header: %v", err)
	}

	// Skip the first byte, because rangeEnd is inclusive
	if rangeStart > 0 {
		io.ReadFull(resp.Body, []byte{0})
	}

	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		return fmt.Errorf("failed to copy response body: %v", err)
	}

	l.offset = rangeEnd
	return nil
}

func jobStatusDone(status buildssrht.JobStatus) bool {
	switch status {
	case buildssrht.JobStatusPending, buildssrht.JobStatusQueued, buildssrht.JobStatusRunning:
		return false
	default:
		return true
	}
}

func jobStatusIcon(status buildssrht.JobStatus) string {
	switch status {
	case buildssrht.JobStatusPending, buildssrht.JobStatusQueued, buildssrht.JobStatusRunning:
		return "⌛"
	case buildssrht.JobStatusSuccess:
		return "✔"
	case buildssrht.JobStatusFailed:
		return "✗"
	case buildssrht.JobStatusTimeout:
		return "⏱️"
	case buildssrht.JobStatusCancelled:
		return "🛑"
	default:
		panic(fmt.Sprintf("unknown job status: %q", status))
	}
}
