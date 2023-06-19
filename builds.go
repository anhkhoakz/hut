package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/juju/ansiterm/tabwriter"
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
	cmd.AddCommand(newBuildsArtifactsCommand())
	cmd.AddCommand(newBuildsUserWebhookCommand())
	return cmd
}

const buildsSubmitPrefill = `

# Please write a build manifest above. The build manifest reference is
# available at:
# https://man.sr.ht/builds.sr.ht/manifest.md
`

func newBuildsSubmitCommand() *cobra.Command {
	var follow, edit bool
	var note, tagString, visibility string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("builds", cmd)

		buildsVisibility, err := buildssrht.ParseVisibility(visibility)
		if err != nil {
			log.Fatal(err)
		}

		filenames := args
		if len(args) == 0 {
			if _, err := os.Stat(".build.yml"); err == nil {
				filenames = append(filenames, ".build.yml")
			}
			if matches, err := filepath.Glob(".builds/*.yml"); err == nil {
				filenames = append(filenames, matches...)
			}
		}

		if len(filenames) == 0 && !edit {
			log.Fatal("no build manifest found")
		}
		if len(filenames) > 1 && follow {
			log.Fatal("--follow cannot be used when submitting multiple jobs")
		}

		tags := strings.Split(tagString, "/")

		var manifests []string
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

			manifests = append(manifests, string(b))
		}

		if edit {
			if len(manifests) == 0 {
				manifests = append(manifests, buildsSubmitPrefill)
			}

			for i, manifest := range manifests {
				var err error
				manifests[i], err = getInputWithEditor("hut*.yml", manifest)
				if err != nil {
					log.Fatal(err)
				}
			}
		}

		for _, manifest := range manifests {
			job, err := buildssrht.Submit(c.Client, ctx, manifest, tags, &note, &buildsVisibility)
			if err != nil {
				log.Fatal(err)
			}

			if termfmt.IsTerminal() {
				log.Printf("Started build %v/%v/job/%v", c.BaseURL, job.Owner.CanonicalName, job.Id)
			} else {
				fmt.Printf("%v/%v/job/%v\n", c.BaseURL, job.Owner.CanonicalName, job.Id)
			}

			if follow {
				id := job.Id
				job, err := followJob(ctx, c, job.Id)
				if err != nil {
					log.Fatal(err)
				}
				if job.Status != buildssrht.JobStatusSuccess {
					offerSSHConnection(ctx, c, id)
				}
			}
		}
	}

	cmd := &cobra.Command{
		Use:               "submit [manifest...]",
		Short:             "Submit a build manifest",
		ValidArgsFunction: cobra.FixedCompletions([]string{"yml", "yaml"}, cobra.ShellCompDirectiveFilterFileExt),
		Run:               run,
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow build logs")
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "edit manifest")
	cmd.Flags().StringVarP(&note, "note", "n", "", "short job description")
	cmd.RegisterFlagCompletionFunc("note", cobra.NoFileCompletions)
	cmd.Flags().StringVarP(&tagString, "tags", "t", "", "job tags (slash separated)")
	cmd.RegisterFlagCompletionFunc("tags", cobra.NoFileCompletions)
	cmd.Flags().StringVarP(&visibility, "visibility", "v", "unlisted", "builds visibility")
	cmd.RegisterFlagCompletionFunc("visibility", completeVisibility)
	return cmd
}

func newBuildsResubmitCommand() *cobra.Command {
	var follow, edit bool
	var note, visibility string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		id, instance, err := parseBuildID(args[0])
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("builds", cmd, instance)

		oldJob, err := buildssrht.Manifest(c.Client, ctx, id)
		if err != nil {
			log.Fatalf("failed to get build manifest: %v", err)
		} else if oldJob == nil {
			log.Fatal("failed to get build manifest: invalid job ID")
		}

		var buildsVisibility buildssrht.Visibility
		if visibility != "" {
			var err error
			buildsVisibility, err = buildssrht.ParseVisibility(visibility)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			buildsVisibility = oldJob.Visibility
		}

		if edit {
			content, err := getInputWithEditor("hut*.yml", oldJob.Manifest)
			if err != nil {
				log.Fatal(err)
			}

			oldJob.Manifest = content
		}

		if note == "" {
			note = fmt.Sprintf("Resubmission of build [#%d](/%s/job/%d)",
				id, oldJob.Owner.CanonicalName, id)
			if edit {
				note += " (edited)"
			}
		}

		job, err := buildssrht.Submit(c.Client, ctx, oldJob.Manifest, nil, &note, &buildsVisibility)
		if err != nil {
			log.Fatal(err)
		}

		if termfmt.IsTerminal() {
			log.Printf("Started build %v/%v/job/%v", c.BaseURL, job.Owner.CanonicalName, job.Id)
		} else {
			fmt.Printf("%v/%v/job/%v\n", c.BaseURL, job.Owner.CanonicalName, job.Id)
		}

		if follow {
			id := job.Id
			job, err := followJob(ctx, c, job.Id)
			if err != nil {
				log.Fatal(err)
			}
			if job.Status != buildssrht.JobStatusSuccess {
				offerSSHConnection(ctx, c, id)
			}
		}
	}

	cmd := &cobra.Command{
		Use:               "resubmit <ID>",
		Short:             "Resubmit a build",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAnyJobs,
		Run:               run,
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow build logs")
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "edit manifest")
	cmd.Flags().StringVarP(&note, "note", "n", "", "short job description")
	cmd.RegisterFlagCompletionFunc("note", cobra.NoFileCompletions)
	cmd.Flags().StringVarP(&visibility, "visibility", "v", "", "builds visibility")
	cmd.RegisterFlagCompletionFunc("visibility", completeVisibility)
	return cmd
}

func newBuildsCancelCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		for _, arg := range args {
			id, instance, err := parseBuildID(arg)
			if err != nil {
				log.Fatal(err)
			}

			c := createClientWithInstance("builds", cmd, instance)

			job, err := buildssrht.Cancel(c.Client, ctx, id)
			if err != nil {
				log.Fatalf("failed to cancel job %d: %v", id, err)
			}

			log.Printf("%d is cancelled\n", job.Id)
		}
	}

	cmd := &cobra.Command{
		Use:               "cancel <ID...>",
		Short:             "Cancel jobs",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeRunningJobs,
		Run:               run,
	}
	return cmd
}

func newBuildsShowCommand() *cobra.Command {
	var follow bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var (
			id int32
			c  *Client
		)
		if len(args) == 0 {
			// get last build
			c = createClient("builds", cmd)
			jobs, err := buildssrht.JobIDs(c.Client, ctx)
			if err != nil {
				log.Fatal(err)
			}
			if len(jobs.Results) == 0 {
				log.Fatal("cannot show last job: no jobs found")
			}
			id = jobs.Results[0].Id
		} else {
			var (
				instance string
				err      error
			)
			id, instance, err = parseBuildID(args[0])
			if err != nil {
				log.Fatal(err)
			}
			c = createClientWithInstance("builds", cmd, instance)
		}

		var (
			err error
			job *buildssrht.Job
		)

		if follow {
			job, err = followJobShow(ctx, c, id)
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

		printJob(os.Stdout, job)

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
				if err := fetchJobLogs(ctx, c, new(buildLog), job); err != nil {
					log.Fatalf("failed to fetch job logs: %v", err)
				}
			} else {
				name := job.Tasks[failedTask].Name
				fmt.Printf("\n%s log:\n", name)
				if err := fetchTaskLogs(ctx, c, new(buildLog), job.Tasks[failedTask]); err != nil {
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
		ValidArgsFunction: completeAnyJobs,
		Run:               run,
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow job status")
	return cmd
}

func newBuildsListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("builds", cmd)
		var cursor *buildssrht.Cursor
		var username string
		if len(args) > 0 {
			username = strings.TrimLeft(args[0], ownerPrefixes)
		}
		pagerify(func(p pager) bool {
			var jobs *buildssrht.JobCursor
			if len(username) > 0 {
				user, err := buildssrht.JobsByUser(c.Client, ctx, username)
				if err != nil {
					log.Fatal(err)
				} else if user == nil {
					log.Fatal("no such user")
				}
				jobs = user.Jobs
			} else {
				var err error
				jobs, err = buildssrht.Jobs(c.Client, ctx, nil)
				if err != nil {
					log.Fatal(err)
				}
			}

			for _, job := range jobs.Results {
				printJob(p, &job)
			}

			cursor = jobs.Cursor
			return cursor == nil
		})
	}

	cmd := &cobra.Command{
		Use:               "list [owner]",
		Short:             "List jobs",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func newBuildsSSHCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		id, instance, err := parseBuildID(args[0])
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("builds", cmd, instance)

		job, ver, err := buildssrht.GetSSHInfo(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if job == nil {
			log.Fatalf("no such job with ID %d", id)
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
		ValidArgsFunction: completeRunningJobs,
		Run:               run,
	}
	return cmd
}

func newBuildsArtifactsCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		id, instance, err := parseBuildID(args[0])
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("builds", cmd, instance)

		job, err := buildssrht.Artifacts(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if job == nil {
			log.Fatalf("no such job with ID %d", id)
		}

		if len(job.Artifacts) == 0 {
			log.Println("No artifacts for this job.")
			return
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer tw.Flush()
		for _, artifact := range job.Artifacts {
			name := artifact.Path[strings.LastIndex(artifact.Path, "/")+1:]
			s := fmt.Sprintf("%s\t%s\t", termfmt.Bold.String(name), humanize.Bytes(uint64(artifact.Size)))
			if artifact.Url == nil {
				s += termfmt.Dim.Sprint("(pruned after 90 days)")
			} else {
				s += *artifact.Url
			}
			fmt.Fprintln(tw, s)
		}
	}

	cmd := &cobra.Command{
		Use:               "artifacts <ID>",
		Short:             "List artifacts",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAnyJobs,
		Run:               run,
	}
	return cmd
}

func newBuildsUserWebhookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user-webhook",
		Short: "Manage user webhooks",
	}
	cmd.AddCommand(newBuildsUserWebhookCreateCommand())
	cmd.AddCommand(newBuildsUserWebhookListCommand())
	cmd.AddCommand(newBuildsUserWebhookDeleteCommand())
	return cmd
}

func newBuildsUserWebhookCreateCommand() *cobra.Command {
	var events []string
	var stdin bool
	var url string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("builds", cmd)

		var config buildssrht.UserWebhookInput
		config.Url = url

		whEvents, err := buildssrht.ParseUserEvents(events)
		if err != nil {
			log.Fatal(err)
		}
		config.Events = whEvents
		config.Query = readWebhookQuery(stdin)

		webhook, err := buildssrht.CreateUserWebhook(c.Client, ctx, config)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Created user webhook with ID %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "create",
		Short:             "Create a user webhook",
		Args:              cobra.ExactArgs(0),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().StringSliceVarP(&events, "events", "e", nil, "webhook events")
	cmd.RegisterFlagCompletionFunc("events", completeBuildsUserWebhookEvents)
	cmd.MarkFlagRequired("events")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read webhook query from stdin")
	cmd.Flags().StringVarP(&url, "url", "u", "", "payload url")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("url")
	return cmd
}

func newBuildsUserWebhookListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("builds", cmd)

		webhooks, err := buildssrht.UserWebhooks(c.Client, ctx)
		if err != nil {
			log.Fatal(err)
		}

		for _, webhook := range webhooks.Results {
			fmt.Printf("%s %s\n", termfmt.DarkYellow.Sprintf("#%d", webhook.Id), webhook.Url)
		}
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List user webhooks",
		Args:  cobra.ExactArgs(0),
		Run:   run,
	}
	return cmd
}

func newBuildsUserWebhookDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("builds", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		webhook, err := buildssrht.DeleteUserWebhook(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Deleted webhook %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete a user webhook",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeBuildsUserWebhookID,
		Run:               run,
	}
	return cmd
}

func printJob(w io.Writer, job *buildssrht.Job) {
	fmt.Fprint(w, termfmt.DarkYellow.Sprintf("#%d", job.Id))
	if tagString := formatJobTags(job); tagString != "" {
		fmt.Fprintf(w, " - %s", termfmt.Bold.String(tagString))
	}
	fmt.Fprintf(w, ": %s\n", job.Status.TermString())

	for _, task := range job.Tasks {
		fmt.Fprintf(w, "%s %s  ", task.Status.TermIcon(), task.Name)
	}
	fmt.Fprintln(w)

	if job.Note != nil && *job.Note != "" {
		fmt.Fprintln(w, "\n"+indent(strings.TrimSpace(*job.Note), "  "))
	}

	fmt.Fprintln(w)
}

func newBuildsSecretsCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("builds", cmd)

		secrets, err := buildssrht.Secrets(c.Client, ctx)
		if err != nil {
			log.Fatal(err)
		}

		var s string
		for i, secret := range secrets.Results {
			if i != 0 {
				s += "\n"
			}

			created := termfmt.Dim.String(humanize.Time(secret.Created.Time))
			s += fmt.Sprintf("%s\t%s\n", termfmt.DarkYellow.Sprint(secret.Uuid), created)
			if secret.Name != nil && *secret.Name != "" {
				s += fmt.Sprintf("%s\n", *secret.Name)
			}

			switch v := secret.Value.(type) {
			case *buildssrht.SecretFile:
				s += fmt.Sprintf("File: %s %o\n", v.Path, v.Mode)
			case *buildssrht.SSHKey:
				s += "SSH Key\n"
			case *buildssrht.PGPKey:
				s += "PGP Key\n"
			}
		}
		fmt.Print(s)
	}

	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "List secrets",
		Args:  cobra.ExactArgs(0),
		Run:   run,
	}
	return cmd
}

type buildLog struct {
	offset int64
	done   bool
}

func followJob(ctx context.Context, c *Client, id int32) (*buildssrht.Job, error) {
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

		if err := fetchJobLogs(ctx, c, logs[""], job); err != nil {
			return nil, fmt.Errorf("failed to fetch job logs: %v", err)
		}
		for _, task := range job.Tasks {
			if err := fetchTaskLogs(ctx, c, logs[task.Name], task); err != nil {
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

func fetchJobLogs(ctx context.Context, c *Client, l *buildLog, job *buildssrht.Job) error {
	switch job.Status {
	case buildssrht.JobStatusPending, buildssrht.JobStatusQueued:
		return nil
	}

	if err := fetchBuildLogs(ctx, c, l, job.Log.FullURL); err != nil {
		return err
	}

	l.done = jobStatusDone(job.Status)
	return nil
}

func fetchTaskLogs(ctx context.Context, c *Client, l *buildLog, task buildssrht.Task) error {
	switch task.Status {
	case buildssrht.TaskStatusPending, buildssrht.TaskStatusSkipped:
		return nil
	}

	if err := fetchBuildLogs(ctx, c, l, task.Log.FullURL); err != nil {
		return err
	}

	switch task.Status {
	case buildssrht.TaskStatusPending, buildssrht.TaskStatusRunning:
		return nil
	}

	l.done = true
	return nil
}

func fetchBuildLogs(ctx context.Context, c *Client, l *buildLog, url string) error {
	if l.done {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%v-", l.offset))

	resp, err := c.HTTP.Do(req)
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

func parseBuildID(s string) (id int32, instance string, err error) {
	s, _, instance = parseResourceName(s)
	s = strings.TrimPrefix(s, "job/")
	id64, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, "", fmt.Errorf("invalid build ID: %v", err)
	}
	return int32(id64), instance, nil
}

func indent(s, prefix string) string {
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}

func formatJobTags(job *buildssrht.Job) string {
	var s string
	for i, tag := range job.Tags {
		if tag == "" {
			break
		}
		if i > 0 {
			s += "/"
		}
		s += tag
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

func followJobShow(ctx context.Context, c *Client, id int32) (*buildssrht.Job, error) {
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
		fmt.Printf("%v%v: %s with %s", termfmt.ReplaceLine(), termfmt.DarkYellow.Sprintf("#%d", job.Id),
			job.Status.TermString(), taskString)

		if jobStatusDone(job.Status) {
			fmt.Print(termfmt.ReplaceLine())
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

func completeRunningJobs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeJobs(cmd, true)
}

func completeAnyJobs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeJobs(cmd, false)
}

func completeJobs(cmd *cobra.Command, onlyRunning bool) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("builds", cmd)
	var jobList []string

	jobs, err := buildssrht.Jobs(c.Client, ctx, nil)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, job := range jobs.Results {
		// TODO: filter with API
		if onlyRunning && jobStatusDone(job.Status) {
			continue
		}

		if cmd.Name() == "cancel" && hasCmdArg(cmd, strconv.Itoa(int(job.Id))) {
			continue
		}

		str := fmt.Sprintf("%d\t", job.Id)
		if tagString := formatJobTags(&job); tagString != "" {
			str += tagString
		}

		if len(job.Tags) > 0 && job.Note != nil {
			str += " - "
		}

		if job.Note != nil {
			str += *job.Note
		}

		jobList = append(jobList, str)
	}

	return jobList, cobra.ShellCompDirectiveNoFileComp
}

func completeBuildsUserWebhookEvents(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var eventList []string
	events := [1]string{"job_created"}
	set := strings.ToLower(cmd.Flag("events").Value.String())
	for _, event := range events {
		if !strings.Contains(set, event) {
			eventList = append(eventList, event)
		}
	}
	return eventList, cobra.ShellCompDirectiveNoFileComp
}

func completeBuildsUserWebhookID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("builds", cmd)
	var webhookList []string

	webhooks, err := buildssrht.UserWebhooks(c.Client, ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, webhook := range webhooks.Results {
		s := fmt.Sprintf("%d\t%s", webhook.Id, webhook.Url)
		webhookList = append(webhookList, s)
	}

	return webhookList, cobra.ShellCompDirectiveNoFileComp
}

func offerSSHConnection(ctx context.Context, c *Client, id int32) {
	if !termfmt.IsTerminal() {
		os.Exit(1)
	}

	termfmt.Bell()
	if !getConfirmation(fmt.Sprintf("\n%s Do you want to log in with SSH?", termfmt.Red.String("Build failed."))) {
		return
	}

	job, ver, err := buildssrht.GetSSHInfo(c.Client, ctx, id)
	if err != nil {
		log.Fatal(err)
	} else if job == nil {
		log.Fatalf("no such job with ID %d", id)
	}

	err = sshConnection(job, ver.Settings.SshUser)
	if err != nil {
		log.Fatal(err)
	}
}
