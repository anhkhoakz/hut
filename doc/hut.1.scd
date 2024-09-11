hut(1)

# NAME

hut - A CLI tool for sr.ht

# SYNOPSIS

*hut* [commands...] [options...]

# DESCRIPTION

hut is a CLI companion utility to interact with sr.ht.

Resources (such as build jobs, todo tickets, lists patchsets, git repositories,
and so on) can be specified in multiple forms: name, owner and name, or full
URL. For instance, the repository _hut_ owned by _~emersion_ on _git.sr.ht_ can
be referred to via:

- "hut"
- "~emersion/hut"
- "https://git.sr.ht/~emersion/hut"

Additionally, mailing lists can be referred to by their email address.

_hut_ commands that read input, like *hut graphql* or *hut builds user-webhook
create* read input depending on whether their stdin is on a terminal or not:

- If stdin is not on a terminal, for example, because stdin is redirected from a
  file or from a pipe, _hut_ reads input from stdin.

- Otherwise, if option *--stdin* is specified to the command, _hut_ reads input
  from stdin.

- Otherwise, _hut_ assumes to run in a terminal and starts the command provided
  by environment variable _$EDITOR_ to read input.

# OPTIONS

*-h*, *--help*
	Show help message and quit.
	Can be used after a command to get more information it.

*--config*
	Explicitly select a configuration file that should be used over
	the default configuration.

*--debug*
	Prints the command's underlying GraphQL request to _stderr_.

*--instance*
	Select which sr.ht instance from the config file should be used.
	By default the first one will be selected.

# COMMANDS

*help* <command>
	Help about any command.

*graphql* <service>
	Write a GraphQL query and execute it. The JSON response is written to
	stdout. _service_ is the sr.ht service to execute the query on (for instance
	"meta" or "builds").

	A tool like *jq*(1) can be used to prettify the output and process the
	data. Example:

	```
	hut graphql meta <<EOF | jq '.me'
	query {
		me { canonicalName }
	}
	EOF
	```

	Options are:

	*--file* <key>=<value>
		Set a file variable.

	*--stdin*
		Read query from _stdin_.

	*-v*, *--var* <key>=<value>
		Set a raw variable. Example:

		```
		hut graphql meta -v username=emersion <<'EOF'
		query($username: String!) {
			userByName(username: $username) {
				bio
			}
		}
		EOF
		```

*init*
	Initialize hut's configuration file.

*export* <directory> [resource|service...]
	Export account data.

	By default, all data of the current user will be exported. Alternatively,
	an explicit list of instance services (e.g. "todo.sr.ht") or resources
	(e.g. "todo.sr.ht/~user/tracker") can be specified.

*import* <directory...>
	Import account data.

## builds

*artifacts* <ID>
	List artifacts.

*cancel* <IDs...>
	Cancel jobs.

*list* [owner]
	List jobs.

*resubmit* <ID>
	Resubmit a build.

	Options are:

	*-e*, *--edit*
		Edit manifest with _$EDITOR_.

	*-f*, *--follow*
		Follow build logs.

	*-n*, *--note* <string>
		Provide a short job description.

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private). Defaults to the same
		visibility used by the original build job.

*secret list*
	List secrets.

*secret share* <secret>
	Share a secret.

	Options are:

	*-u*, *--user*
		User with whom to share the secret (required).

*show* [ID] [options...]
	Show job status.

	If no ID is specified, the latest build will be printed.

	Options are:

	*-f*, *--follow*
		Follow job status.

*ssh* <ID>
	Connect with SSH to a job.

*submit* [manifest...] [options...]
	Submit a build manifest.

	If no build manifest is specified, build manifests are discovered at
	_.build.yml_, _.build.yaml_, _.builds/\*.yml_ and _.builds/\*.yaml_.

	Options are:

	*-e*, *--edit*
		Edit manifest with _$EDITOR_.

	*-f*, *--follow*
		Follow build logs.

	*-n*, *--note* <string>
		Provide a short job description.

	*-t*, *--tags* <string>
		Slash separated tags (e.g. "hut/test").

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private). Defaults to unlisted.

*user-webhook create* [options...]
	Create a user webhook.

	Options are:

	*-e*, *--events* <strings...>
		List of events that should trigger the webhook (JOB_CREATED). Required.

	*--stdin*
		Read query from _stdin_.

	*-u*, *--url* <URL>
		The payload URL which receives the _POST_ request. Required.

*user-webhook delete* <ID>
	Delete a user webhook.

*user-webhook list*
	List user webhooks.

## git

Options are:

	*-r*, *--repo* <string>
		Name of repository.

*acl delete* <ID>
	Delete an ACL entry.

*acl list* [repo]
	List ACL entries of a repo. Defaults to current repo.

*acl update* <user> [options...]
	Update or add an ACL entry for user.

	Options are:

	*-m*, *--mode* <string>
		Access mode to set (RW, RO).

*artifact delete* <ID>
	Delete an artifact.

*artifact list* [options...]
	List artifacts.

*artifact upload* <filename...> [options...]
	Upload artifacts.

	Options are:

	*--rev* <string>
		Revision tag. Defaults to the last Git tag.

*clone* <URL>
	This will clone the repository to _CWD_ and try to configure it for 
	_git send-email_ if possible.

*create* <name> [options...]
	Create a repository. If *--clone* is not used, the remote URL will be
	printed to _stdout_.

	Options are:

	*-c*, *--clone*
		Clone repository to _CWD_.

	*-d*, *--description* <string>
		Description of the repository.

	*--import-url* <url>
		Import the repository from the given URL.

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private). Defaults to public.

*delete* [repo] [options...]
	Delete a repository. By default the current repo will be deleted.

	Options are:

	*-y*, *--yes*
		Confirm deletion without prompt.

*list* [owner]
	List repositories.

*setup* [options...]
	Setup a repository for _git send-email_. hut will read the required
	settings from the project configuration file. If the repository already
	has some configuration, the setup will be skipped.

	Options are:

	*-f*, *--force*
		Force the setup even if the repository is already configured for
		_git send-email_.

*show* [repo]
	Display information about a repository.

*update* [repo] [options...]
	Update a repository. By default the current repo will be updated.

	Options are:

	*-b*, *--default-branch* <branch>
		Set the default branch.

	*-d*, *--description* <description>
		Set one-line repository description.

	*--readme* <file>
		Update the custom README settings. You can read the HTML from a file or
		pass "-" as the filename to read from _stdin_.
		To clear the custom README use an empty string "".

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private).

*user-webhook create* [options...]
	Create a user webhook.

	Options are:

	*-e*, *--events* <strings...>
		List of events that should trigger the webhook (REPO_CREATED,
		REPO_UPDATE, REPO_DELETED). Required.

	*--stdin*
		Read query from _stdin_.

	*-u*, *--url* <URL>
		The payload URL which receives the _POST_ request. Required.

*user-webhook delete* <ID>
	Delete a user webhook.

*user-webhook list*
	List user webhooks.

## hg

Options are:

	*-r*, *--repo* <string>
		Name of repository.

*acl delete* <ID>
	Delete an ACL entry.

*acl list* [repo]
	List ACL entries of a repo. Defaults to current repo.

*acl update* <user> [options...]
	Update or add an ACL entry for user.

	Options are:

	*-m*, *--mode* <string>
		Access mode to set (RW, RO).

*create* <name> [options...]
	Create a repository. If *--clone* is not used, the remote URL will be
	printed to _stdout_.

	Options are:

	*-c*, *--clone*
		Clone repository to _CWD_.

	*-d*, *--description* <string>
		Description of the repository.

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private). Defaults to public.

*delete* [repo] [options...]
	Delete a repository. By default the current repo will be deleted.

	Options are:

	*-y*, *--yes*
		Confirm deletion without prompt.

*list* [owner]
	List repositories.

*update* [repo] [options...]
	Update a repository. By default the current repo will be updated.

	Options are:

	*-d*, *--description* <description>
		Set one-line repository description.

	*--non-publishing* <boolean>
		Controls whether this repository is a non-publishing repository.

	*--readme* <file>
		Update the custom README settings. You can read the HTML from a file or
		pass "-" as the filename to read from _stdin_.
		To clear the custom README use an empty string "".

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private).

*user-webhook create* [options...]
	Create a user webhook.

	Options are:

	*-e*, *--events* <strings...>
		List of events that should trigger the webhook (REPO_CREATED,
		REPO_UPDATE, REPO_DELETED). Required.

	*--stdin*
		Read query from _stdin_.

	*-u*, *--url* <URL>
		The payload URL which receives the _POST_ request. Required.

*user-webhook delete* <ID>
	Delete a user webhook.

*user-webhook list*
	List user webhooks.

## lists

Options are:

	*-l*, *--mailing-list* <name>
		Select a mailing list.

		By default, the mailing list configured for the current Git repository
		will be selected.

*acl delete* <ID>
	Delete an ACL entry.

*acl list* [list]
	List ACL entries of a mailing list.

*archive* [list] [options...]
	Download a mailing list archive as an mbox file to _stdout_.

	Options are:

	*-d*, *--days* <int>
		Number of last days for which the archive should be downloaded.
		By default the entire archive will be selected.

*create* <name> [options...]
	Create a mailing list.

	Options are:

	*--stdin*
		Read description from _stdin_.

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private). Defaults to public.

*delete* [list] [options...]
	Delete a mailing list.

	Options are:

	*-y*, *--yes*
		Confirm deletion without prompt.

*list* [owner]
	List mailing lists.

*patchset apply* <ID>
	Apply a patchset.

*patchset list* [list] [options...]
	List patchsets in list.

	Options are:

	*-u*, *--user*
		List patchsets by user instead of by list.

*patchset show* <ID>
	Show a patchset.

*patchset update* <ID>
	Update a patchset.

	Options are:

	*-s*, *--status* <string>
		Patchset status to set (required).

*subscribe* [list]
	Subscribe to a mailing list.

*subscriptions*
	List mailing list subscriptions.

*unsubscribe* [list]
	Unsubscribe from a mailing list.

*update* [list] [options...]
	Update a mailing list.

	Options are:

	*--description*
		Edit description.

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private).

*user-webhook create* [options...]
	Create a user webhook.

	Options are:

	*-e*, *--events* <strings...>
		List of events that should trigger the webhook (LIST_CREATED,
		LIST_UPDATED, LIST_DELETED, EMAIL_RECEIVED, PATCHSET_RECEIVED).
		Required.

	*--stdin*
		Read query from _stdin_.

	*-u*, *--url* <URL>
		The payload URL which receives the _POST_ request. Required.

*user-webhook delete* <ID>
	Delete a user webhook.

*user-webhook list*
	List user webhooks.

*webhook create* [list] [options...]
	Create a mailing list webhook.

	Options are:

	*-e*, *--events* <strings...>
		List of events that should trigger the webhook (LIST_UPDATED,
		LIST_DELETED, EMAIL_RECEIVED, PATCHSET_RECEIVED). Required.

	*--stdin*
		Read query from _stdin_.

	*-u*, *--url* <URL>
		The payload URL which receives the _POST_ request. Required.

*webhook delete* <ID>
	Delete a tracker webhook.

*webhook list* [list]
	List mailing list webhooks.

## meta

*audit-log*
	Display your audit log.

*oauth tokens*
	List personal access tokens.

*pgp-key create* [path]
	Upload a PGP public key and associate it with your account.

	The public key must be in the armored format.

	If _path_ is not specified, the default public key from the local GPG
	keyring is used.

*pgp-key delete* <ID>
	Delete a PGP key from your account.

*pgp-key list* [username] [options...]
	List PGP public keys.

	Options are:

	*-r*, *--raw*
		Only print raw public key

*show* [username]
	Show a user's profile.

	If _username_ is not specified, your profile is displayed.

*ssh-key create* [path]
	Upload an SSH public key and associate it with your account.

	If _path_ is not specified, the default SSH public key is used.

*ssh-key delete* <ID>
	Delete an SSH public key from your account.

*ssh-key list* [username] [options...]
	List SSH public keys.

	Options are:

	*-r*, *--raw*
		Only print raw public key

*update* [options...]
	Update account.

	Options are:

	*--bio*
		Edit biography.

	*--email* <string>
		Set Email address.

	*--location* <string>
		Set location.

	*--url* <string>
		Set URL.

*user-webhook create* [options...]
	Create a user webhook.

	Options are:

	*-e*, *--events* <strings...>
		List of events that should trigger the webhook (PROFILE_UPDATE,
		PGP_KEY_ADDED, PGP_KEY_REMOVED, SSH_KEY_ADDED, SSH_KEY_REMOVED).
		Required.

	*--stdin*
		Read query from _stdin_.

	*-u*, *--url* <URL>
		The payload URL which receives the _POST_ request. Required.

*user-webhook delete* <ID>
	Delete a user webhook.

*user-webhook list*
	List user webhooks.

## pages

*list*
	List registered sites.

*publish* [file] [options...]
	Publish a website.

	The input _file_ can be either a gzip tarball or a directory. If _file_ is
	not specified, standard input is used.

	Options are:

	*-d*, *--domain* <string>
		Fully qualified domain name.

	*-p*, *--protocol* <string>
		Protocol to use (either HTTPS or GEMINI; defaults to HTTPS)

	*--site-config* <string>
		Path to site configuration file (for e.g. cache-control).

	*-s*, *--subdirectory* <string>
		If specified, only this subdirectory is updated, the rest of the site
		is left untouched.

*unpublish* [options...]
	Unpublish a website.

	Options are:

	*-d*, *--domain* <string>
		Fully qualified domain name.

	*-p*, *--protocol* <string>
		Protocol to use (either HTTPS or GEMINI; defaults to HTTPS)

*user-webhook create* [options...]
	Create a user webhook.

	Options are:

	*-e*, *--events* <strings...>
		List of events that should trigger the webhook (SITE_PUBLISHED,
		SITE_UNPUBLISHED). Required.

	*--stdin*
		Read query from _stdin_.

	*-u*, *--url* <URL>
		The payload URL which receives the _POST_ request. Required.

*user-webhook delete* <ID>
	Delete a user webhook.

*user-webhook list*
	List user webhooks.

## paste

*create* <filenames...>
	Create a new paste.

	Options are:

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private). Defaults to unlisted.

	*-n*, *--name* <string>
		Name of the created paste. Only valid when reading from stdin.

*delete* <IDs...>
	Delete pastes.

*list*
	List pastes.

*show* <ID>
	Display a paste.

*update* <ID> [options...]
	Update a paste's visibility.

	Options are:

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private)

*user-webhook create* [options...]
	Create a user webhook.

	Options are:

	*-e*, *--events* <strings...>
		List of events that should trigger the webhook (PASTE_CREATED,
		PASTE_UPDATED, PASTE_DELETED). Required.

	*--stdin*
		Read query from _stdin_.

	*-u*, *--url* <URL>
		The payload URL which receives the _POST_ request. Required.

*user-webhook delete* <ID>
	Delete a user webhook.

*user-webhook list*
	List user webhooks.

## todo

Options are:

	*-t*, *--tracker* <string>
		Name of tracker.

*acl delete* <ID>
	Delete an ACL entry.

*acl list* [tracker]
	List ACL entries of a tracker.

*create* <name> [options...]
	Create a tracker.

	Options are:

	*--stdin*
		Read description from _stdin_.

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private). Defaults to public.

*delete* [tracker] [options...]
	Delete a tracker.

	Options are:

	*-y*, *--yes*
		Confirm deletion without prompt.

*label create* <name> [options...]
	Create a label.

	Options are:

	*-b*, *--background*
		Background color in hex format (required).

	*-f*, *--foreground*
		Foreground color in hex format. If omitted either black or white will be
		selected for an optimized contrast.

*label delete* <name>
	Delete a label.

*label list*
	List labels.

*label update* <name> [options...]
	Update a label.

	Options are:

	*-b*, *--background*
		Background color in hex format.

	*-f*, *--foreground*
		Foreground color in hex format.

	*-n*, *--name*
		New label name.

*list* [owner]
	List trackers.

*subscribe* [tracker]
	Subscribe to a tracker.

*ticket assign* <ID> [options...]
	Assign a user to a ticket.

	Options are:

	*-u*, *--user*
		Username of the new assignee (required).

*ticket comment* <ID> [options...]
	Comment on a ticket.

	Options are:

	*-r*, *--resolution*
		Resolution for resolved tickets. If status is omitted, it will be set to
		_RESOLVED_.

	*-s*, *--status*
		New ticket status. If set to _RESOLVED_, resolution will default to
		_CLOSED_.

	*--stdin*
		Read comment from _stdin_.

*ticket create* [options...]
	Create a new ticket.

	Options are:

	*--stdin*
		Read ticket from _stdin_.

*ticket delete* <ID> [options...]
	Delete a ticket.

	Options are:

	*-y*, *--yes*
		Confirm deletion without prompt.

*ticket edit* <ID>
	Edit a ticket.

*ticket label* <ID> [options...]
	Add a label to a ticket.

	Options are:

	*-l*, *--label* <name>
		Name of the label (required).

*ticket list* [options...]
	List tickets.

	Options are:

	*-s*, *--status* <string>
		Filter by ticket status.

*ticket show* <ID>
	Display a ticket.

*ticket subscribe* <ID>
	Subscribe to a ticket.

*ticket unassign* <ID> [options...]
	Unassign a user from a ticket.

	Options are:

	*-u*, *--user*
		Username of the assignee (required).

*ticket unlabel* <ID> [options...]
	Remove a label from a ticket.

	Options are:

	*-l*, *--label* <name>
		Name of the label (required).

*ticket unsubscribe* <ID>
	Unsubscribe from a ticket.

*ticket update-status* <ID> [options...]
	Update status of a ticket.

	Options are:

	*-r*, *--resolution*
		Resolution for resolved tickets (required if status _RESOLVED_ is used).
		If status is omitted, it will be set to _RESOLVED_.

	*-s*, *--status*
		New ticket status.

*ticket webhook create* <ID> [options...]
	Create a ticket webhook.

	Options are:

	*-e*, *--events* <strings...>
		List of events that should trigger the webhook (EVENT_CREATED,
		TICKET_UPDATE, TICKET_DELETED). Required.

	*--stdin*
		Read query from _stdin_.

	*-u*, *--url* <URL>
		The payload URL which receives the _POST_ request. Required.

*ticket webhook delete* <ID>
	Delete a ticket webhook.

*ticket webhook list* <ID>
	List ticket webhooks.

*unsubscribe* [tracker]
	Unsubscribe from a tracker.

*update* [tracker] [options...]
	Update a tracker.

	Options are:

	*--description*
		Edit description.

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private).

*user-webhook create* [options...]
	Create a user webhook.

	Options are:

	*-e*, *--events* <strings...>
		List of events that should trigger the webhook (TRACKER_CREATED,
		TRACKER_UPDATE, TRACKER_DELETED, TICKET_CREATED). Required.

	*--stdin*
		Read query from _stdin_.

	*-u*, *--url* <URL>
		The payload URL which receives the _POST_ request. Required.

*user-webhook delete* <ID>
	Delete a user webhook.

*user-webhook list*
	List user webhooks.

*webhook create* [tracker] [options...]
	Create a tracker webhook.

	Options are:

	*-e*, *--events* <strings...>
		List of events that should trigger the webhook (TRACKER_UPDATE,
		TRACKER_DELETED, LABEL_CREATED, LABEL_UPDATE, LABEL_DELETED,
		TICKET_CREATED, TICKET_UPDATE, TICKET_DELETED, EVENT_CREATED). Required.

	*--stdin*
		Read query from _stdin_.

	*-u*, *--url* <URL>
		The payload URL which receives the _POST_ request. Required.

*webhook delete* <ID>
	Delete a tracker webhook.

*webhook list* [tracker]
	List tracker webhooks.

# CONFIGURATION

Generate a new OAuth2 access token on _meta.sr.ht_.

On startup hut will look for a file at *$XDG_CONFIG_HOME/hut/config*. If
unset, _$XDG_CONFIG_HOME_ defaults to *~/.config/*.

```
instance "sr.ht" {
	access-token "<token>"
	# As an alternative you can specify a command whose first line of output
	# will be parsed as the token
	access-token-cmd pass token
	meta {
		# You can set the origin for each service. As fallback hut will
		# construct the origin from the instance name and the service.
		origin "https://meta.sr.ht"
	}
}
```

# Project configuration file

The project configuration file is a top-level file called _.hut.scfg_ in a
repository, where the assosciated tracker and development mailing list can be
specified. These resources will be used whenever a command is called which needs
a tracker/mailing list and none is explicitly set. Furthermore it is possible
to configure that patches should contain the repository name in their prefix.
The _hut git clone_ command will configure a freshly cloned repository to make
contributing easier.

```
tracker https://todo.sr.ht/~xenrox/hut
development-mailing-list ~xenrox/hut-dev@lists.sr.ht
patch-prefix false
```

# AUTHORS

Originally written by Simon Ser <contact@emersion.fr>. Currently maintained
by Thorben Günther <admin@xenrox.net>, who is assisted by other open-source
contributors. For more information about hut development, see
<https://sr.ht/~xenrox/hut>.
