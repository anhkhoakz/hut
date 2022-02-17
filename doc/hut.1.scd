hut(1)

# NAME

hut - A CLI tool for sr.ht

# SYNOPSIS

*hut* [commands...] [options...]

# DESCRIPTION

hut is a CLI companion utility to interact with sr.ht.

# OPTIONS

*--config*
	Explicitly select a configuration file that should be used over
	the default configuration.

*-h*, *--help*
	Show help message and quit.
	Can be used after a command to get more information it.

*--instance*
	Select which sr.ht instance from the config file should be used.
	By default the first one will be selected.

# COMMANDS

*help* <command>
	Help about any command.

*graphql* <service>
	Read a GraphQL query from stdin and execute it. The JSON response is
	written to stdout. _service_ is the sr.ht service to execute to query on
	(for instance "meta" or "builds").

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

## git

Options are:

	*-r*, *--repo* <string>
		Name of repository.

*create* <name> [options...]
	Create a repository.

	Options are:

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private). Defaults to unlisted.

	*-d*, *--description* <string>
		Description of the repository.

	*-c*, *--clone*
		Clone repository to _CWD_.

*list* [owner]
	List repositories.

*delete* [repo] [options...]
	Delete a repository. By default the current repo will be deleted.

	Options are:

	*-y*, *--yes*
		Confirm deletion without prompt.

*artifact upload* <filename> [options...]
	Upload an artifact.

	Options are:

	*--rev* <string>
		Revision tag. Defaults to the last Git tag.

*artifact list* [options...]
	List artifacts.

*artifact delete* <ID>
	Delete an artifact.

*acl list* [repo]
	List ACL entries of a repo. Defaults to current repo.

*acl update* <user> [options...]
	Update or add an ACL entry for user.

	Options are:

	*-m*, *--mode* <string>
		Access mode to set (RW, RO).

*acl delete* <ID>
	Delete an ACL entry.

## builds

*submit* [manifest...] [options...]
	Submit a build manifest.

	If no build manifest is specified, build manifests are discovered at
	_.build.yml_ and _.builds/\*.yml_.

	Options are:

	*-f*, *--follow*
		Follow build logs.

*resubmit* <ID>
	Resubmit a build.

	Options are:

	*-f*, *--follow*
		Follow build logs.

	*-e*, *--edit*
		Edit manifest with _$EDITOR_.

*cancel* <IDs...>
	Cancel jobs.

*show* [ID] [options...]
	Show job status.

	If no ID is specified, the latest build will be printed.

	Options are:

	*-f*, *--follow*
		Follow job status.

*list*
	List jobs.

*secrets*
	List secrets.

*ssh* <ID>
	Connect with SSH to a job.

## meta

*show* [username]
	Show a user's profile.

	If _username_ is not specified, your profile is displayed.

*audit-log*
	Display your audit log.

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

## paste

*create* <filenames...>
	Create a new paste.

	Options are:

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private)

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

## pages

*publish* <tarball> [options...]
	Publish a website.

	Options are:

	*-d*, *--domain* <string>
		Fully qualified domain name.

	*-p*, *--protocol* <string>
		Protocol to use (either HTTPS or GEMINI; defaults to HTTPS)

*unpublish* [options...]
	Unpublish a website.

	Options are:

	*-d*, *--domain* <string>
		Fully qualified domain name.

	*-p*, *--protocol* <string>
		Protocol to use (either HTTPS or GEMINI; defaults to HTTPS)

*list*
	List registered sites.

## lists

Options are:

	*-l*, *--mailing-list* <name>
		Select a mailing list.

		By default, the mailing list configured for the current Git repository
		will be selected.

*delete* [list] [options...]
	Delete a mailing list.

	Options are:

	*-y*, *--yes*
		Confirm deletion without prompt.

*list* [owner]
	List mailing lists.

*subscribe* [list]
	Subscribe to a mailing list.

*unsubscribe* [list]
	Unsubscribe from a mailing list.

*patchset list* [list] [options...]
	List patchsets in list.

	Options are:

	*-u*, *--user*
		List patchsets by user instead of by list.

*patchset update* <ID>
	Update a patchset.

	Options are:

	*-s*, *--status* <string>
		Patchset status to set (required).

*patchset apply* <ID>
	Apply a patchset.

*acl list* [list]
	List ACL entries of a mailing list.

*acl delete* <ID>
	Delete an ACL entry.

## hg

*list* [owner]
	List repositories.

## todo

Options are:

	*-t*, *--tracker* <string>
		Name of tracker.

*list* [owner]
	List trackers.

*delete* <tracker> [options...]
	Delete a tracker.

	Options are:

	*-y*, *--yes*
		Confirm deletion without prompt.

*ticket list*
	List tickets.

*ticket comment* <ID>
	Comment on a ticket.

*ticket update-status* <ID> [options...]
	Update status of a ticket.

	Options are:

	*-s*, *--status*
		New ticket status (required).

	*-r*, *--resolution*
		Resolution for resolved tickets (required if status _RESOLVED_ is used).

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
}
```

# AUTHORS

Maintained by Simon Ser <contact@emersion.fr>, who is assisted by other
open-source contributors. For more information about hut development, see
<https://sr.ht/~emersion/hut>.
