hut(1)

# NAME

hut - A CLI tool for sr.ht

# SYNOPSIS

*hut* [commands...] [options...]

# DESCRIPTION

hut is a CLI companion utility to interact with sr.ht.

# OPTIONS

*-h*, *--help*
	Show help message and quit.
	Can be used after a command to get more information it.

*--config*
	Explicitly select a configuration file that should be used over
	the default configuration.

*--instance*
	Select which sr.ht instance from the config file should be used.
	By default the first one will be selected.

# COMMANDS

*help* <command>
	Help about any command.

*graphql* <service>
	Write a GraphQL query and execute it. The JSON response is written to
	stdout. _service_ is the sr.ht service to execute the query on (for instance
	"meta" or "builds"). When this command is run from a terminal, the query
	will be read from _$EDITOR_, otherwise it defaults to stdin.

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

*export* <directory>
	Export account data.

	Note, only meta.sr.ht is supported at the moment.

## builds

*cancel* <IDs...>
	Cancel jobs.

*list*
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

	*-t*, *--tags* <string>
		Slash separated tags (e.g. "hut/test").

*secrets*
	List secrets.

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
	_.build.yml_ and _.builds/\*.yml_.

	Options are:

	*-f*, *--follow*
		Follow build logs.

	*-n*, *--note* <string>
		Provide a short job description.

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

*create* <name> [options...]
	Create a repository.

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

*show* [repo]
	Display information about a repository.

*update* [repo] [options...]
	Update a repository.

	Options are:

	*-b*, *--default-branch* <branch>
		Set the default branch.

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private).

*user-webhook create* [options...]
	Create a user webhook. When this command is run from a terminal, the query will
	be read from _$EDITOR_, otherwise it defaults to stdin.

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

*create* <name> [options...]
	Create a repository.

	Options are:

	*-d*, *--description* <string>
		Description of the repository.

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private). Defaults to public.

*delete* <repo> [options...]
	Delete a repository.

	Options are:

	*-y*, *--yes*
		Confirm deletion without prompt.

*list* [owner]
	List repositories.

*user-webhook create* [options...]
	Create a user webhook. When this command is run from a terminal, the query will
	be read from _$EDITOR_, otherwise it defaults to stdin.

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

*unsubscribe* [list]
	Unsubscribe from a mailing list.

*user-webhook create* [options...]
	Create a user webhook. When this command is run from a terminal, the query will
	be read from _$EDITOR_, otherwise it defaults to stdin.

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

## meta

*audit-log*
	Display your audit log.

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

*user-webhook create* [options...]
	Create a user webhook. When this command is run from a terminal, the query will
	be read from _$EDITOR_, otherwise it defaults to stdin.

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

*publish* [tarball] [options...]
	Publish a website.

	If _tarball_ is not specified, standard input is used.

	Options are:

	*-d*, *--domain* <string>
		Fully qualified domain name.

	*--not-found* <string>
		Path to serve for page not found responses

	*-p*, *--protocol* <string>
		Protocol to use (either HTTPS or GEMINI; defaults to HTTPS)

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
	Create a user webhook. When this command is run from a terminal, the query will
	be read from _$EDITOR_, otherwise it defaults to stdin.

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
	Create a user webhook. When this command is run from a terminal, the query will
	be read from _$EDITOR_, otherwise it defaults to stdin.

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

*delete* <tracker> [options...]
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

*label delete* <ID>
	Delete a label.

*label list*
	List labels.

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
	Comment on a ticket with _$EDITOR_.

	Options are:

	*-r*, *--resolution*
		Resolution for resolved tickets. If status is omitted, it will be set to
		_RESOLVED_.

	*-s*, *--status*
		New ticket status. If set to _RESOLVED_, resolution will default to
		_CLOSED_.

	*--stdin*
		Read comment from _stdin_.

*ticket delete* <ID> [options...]
	Delete a ticket.

	Options are:

	*-y*, *--yes*
		Confirm deletion without prompt.

*ticket list*
	List tickets.

*ticket subscribe* <ID>
	Subscribe to a ticket.

*ticket unassign* <ID> [options...]
	Unassign a user from a ticket.

	Options are:

	*-u*, *--user*
		Username of the assignee (required).

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

*user-webhook create* [options...]
	Create a user webhook. When this command is run from a terminal, the query will
	be read from _$EDITOR_, otherwise it defaults to stdin.

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

# AUTHORS

Maintained by Simon Ser <contact@emersion.fr>, who is assisted by other
open-source contributors. For more information about hut development, see
<https://sr.ht/~emersion/hut>.
