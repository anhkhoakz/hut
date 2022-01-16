package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"git.sr.ht/~emersion/hut/srht/buildssrht"
	"git.sr.ht/~emersion/hut/termfmt"
)

func newBuildsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "builds",
		Short: "Use the builds API",
	}
	cmd.AddCommand(newBuildsSubmitCommand())
	cmd.AddCommand(newBuildsResubmitCommand())
	cmd.AddCommand(newBuildsCancelCommand())
	cmd.AddCommand(newBuildsShowCommand())
	cmd.AddCommand(newBuildsListCommand())
	cmd.AddCommand(newBuildsSecretsCommand())
	cmd.AddCommand(newBuildsSSHCommand())
	return cmd
}

func newBuildsSubmitCommand() *cobra.Command {
	var follow bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("builds", cmd)

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

			job, err := buildssrht.Submit(c.Client, ctx, string(b), nil)
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
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{"yml", "yaml"}, cobra.ShellCompDirectiveFilterFileExt
		},
		Run: run,
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow build logs")
	return cmd
}

func newBuildsResubmitCommand() *cobra.Command {
	var follow, edit bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("builds", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		oldJob, err := buildssrht.Manifest(c.Client, ctx, id)
		if err != nil {
			log.Fatalf("failed to get build manifest: %v", err)
		} else if oldJob == nil {
			log.Fatal("failed to get build manifest: invalid job ID")
		}

		if edit {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				log.Fatal("EDITOR not set")
			}

			file, err := ioutil.TempFile(os.TempDir(), "hut*.yml")
			if err != nil {
				log.Fatal(err)
			}
			defer os.Remove(file.Name())

			_, err = file.WriteString(oldJob.Manifest)
			if err != nil {
				log.Fatal(err)
			}

			err = file.Close()
			if err != nil {
				log.Fatal(err)
			}

			cmd := exec.Command(editor, file.Name())
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			err = cmd.Run()
			if err != nil {
				log.Fatal(err)
			}

			content, err := ioutil.ReadFile(file.Name())
			if err != nil {
				log.Fatal(err)
			}

			oldJob.Manifest = string(content)
		}

		note := fmt.Sprintf("Resubmission of build [#%d](/%s/job/%d)",
			id, oldJob.Owner.CanonicalName, id)

		if edit {
			note += " (edited)"
		}

		job, err := buildssrht.Submit(c.Client, ctx, oldJob.Manifest, &note)
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

	cmd := &cobra.Command{
		Use:               "resubmit <ID>",
		Short:             "Resubmit a build",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow build logs")
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "edit manifest")
	return cmd
}

func newBuildsCancelCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("builds", cmd)

		for _, id := range args {
			id, err := parseInt32(id)
			if err != nil {
				log.Fatal(err)
			}

			job, err := buildssrht.Cancel(c.Client, ctx, id)
			if err != nil {
				log.Fatalf("failed to cancel job %d: %v", id, err)
			}

			fmt.Printf("%d is cancelled\n", job.Id)
		}
	}

	cmd := &cobra.Command{
		Use:               "cancel <ID...>",
		Short:             "Cancel jobs",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func newBuildsShowCommand() *cobra.Command {
	var follow bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("builds", cmd)

		// get last build
		var id int32
		if len(args) == 0 {
			jobs, err := buildssrht.JobIDs(c.Client, ctx)
			if err != nil {
				log.Fatal(err)
			}
			if len(jobs.Results) == 0 {
				log.Fatal("cannot show last job: no jobs found")
			}

			id = jobs.Results[0].Id
		} else {
			var err error
			id, err = parseInt32(args[0])
			if err != nil {
				log.Fatal(err)
			}
		}

		var (
			err error
			job *buildssrht.Job
		)

		if follow {
			job, err = c.followJobShow(context.Background(), id)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			job, err = buildssrht.Show(c.Client, ctx, id)
			if err != nil {
				log.Fatal(err)
			} else if job == nil {
				log.Fatal("invalid job ID")
			}
		}

		printJob(job)

		failedTask := -1
		for i, task := range job.Tasks {
			if task.Status == buildssrht.TaskStatusFailed {
				failedTask = i
				break
			}
		}

		if job.Status == buildssrht.JobStatusFailed {
			if failedTask == -1 {
				fmt.Printf("\nSetup log:\n")
				if err := fetchJobLogs(ctx, new(buildLog), job); err != nil {
					log.Fatalf("failed to fetch job logs: %v", err)
				}
			} else {
				name := job.Tasks[failedTask].Name
				fmt.Printf("\n%s log:\n", name)
				if err := fetchTaskLogs(ctx, new(buildLog), job.Tasks[failedTask]); err != nil {
					log.Fatalf("failed to fetch task logs: %v", err)
				}
			}

			cmd, err := getSSHCommand(job)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("\nSSH Command: %s\n", cmd)
		}
	}

	cmd := &cobra.Command{
		Use:               "show [ID]",
		Short:             "Show job status",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow job status")
	return cmd
}

func newBuildsListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("builds", cmd)

		jobs, err := buildssrht.Jobs(c.Client, ctx)
		if err != nil {
			log.Fatal(err)
		}

		for _, job := range jobs.Results {
			printJob(&job)
		}
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List jobs",
		Run:   run,
	}
	return cmd
}

func newBuildsSSHCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("builds", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		job, ver, err := buildssrht.GetSSHInfo(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		err = sshConnection(job, ver.Settings.SshUser)
		if err != nil {
			log.Fatal(err)
		}
	}

	cmd := &cobra.Command{
		Use:               "ssh <ID>",
		Short:             "SSH into job",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func printJob(job *buildssrht.Job) {
	fmt.Print(termfmt.DarkYellow.Sprintf("#%d", job.Id))
	if tagString := formatJobTags(job); tagString != "" {
		fmt.Printf(" - %s", termfmt.Bold.String(tagString))
	}
	fmt.Printf(": %s\n", job.Status.TermString())

	for _, task := range job.Tasks {
		fmt.Printf("%s %s  ", task.Status.TermIcon(), task.Name)
	}
	fmt.Println()

	if job.Note != nil {
		fmt.Println("\n" + indent(strings.TrimSpace(*job.Note), "  "))
	}

	fmt.Println()
}

func newBuildsSecretsCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("builds", cmd)

		secrets, err := buildssrht.Secrets(c.Client, ctx)
		if err != nil {
			log.Fatal(err)
		}

		for _, secret := range secrets.Results {
			if secret.Name != nil {
				fmt.Printf("%s (%s)\n", secret.Uuid, *secret.Name)
			} else {
				fmt.Println(secret.Uuid)
			}
		}
	}

	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "List secrets",
		Run:   run,
	}
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
			fmt.Println(job.Status.TermString())
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

func getSSHCommand(job *buildssrht.Job) (string, error) {
	// TODO: compare timestamps and check if ssh access is still possible
	if job.Runner == nil {
		return "", errors.New("job has no runner assigned yet")
	}

	cmd := fmt.Sprintf("ssh -t builds@%s connect %d", *job.Runner, job.Id)
	return cmd, nil
}

func indent(s, prefix string) string {
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}

func formatJobTags(job *buildssrht.Job) string {
	var s string
	for i, tag := range job.Tags {
		if tag == nil || *tag == "" {
			break
		}
		if i > 0 {
			s += "/"
		}
		s += *tag
	}
	return s
}

func sshConnection(job *buildssrht.Job, user string) error {
	if job.Runner == nil {
		return errors.New("job has no runner assigned yet")
	}

	cmd := exec.Command("ssh", "-t", fmt.Sprintf("%s@%s", user, *job.Runner),
		"connect", fmt.Sprint(job.Id))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (c *Client) followJobShow(ctx context.Context, id int32) (*buildssrht.Job, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		job, err := buildssrht.Show(c.Client, ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to monitor job: %v", err)
		} else if job == nil {
			return nil, errors.New("invalid job ID")
		}

		var taskString string
		for _, task := range job.Tasks {
			taskString += fmt.Sprintf("%s %s ", task.Status.TermIcon(), task.Name)
		}
		fmt.Printf("\x1b[1K\r#%d: %s with %s", job.Id,
			job.Status.TermString(), taskString)

		if jobStatusDone(job.Status) {
			fmt.Print("\x1b[1K\r")
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
