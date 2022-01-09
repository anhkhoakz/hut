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
	Select which srht instance from the config file should be used.
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

## git
	Options are:

	*-r*, *--repo* <string>
		Name of repository.

*create* <repoName> [options...]
	Create a repository.

	Options are:

	*-v*, *--visibility* <string>
		Visibility to use (public, unlisted, private). Defaults to unlisted.

	*-d*, *--description* <string>
		Description of the repository.

	*-c*, *--clone*
		Clone repository to _CWD_.

*list* [username]
	List repositories.

*artifact upload* <filename> [options...]
	Upload an artifact.

	Options are:

	*--rev* <string>
		Revision tag. Defaults to the last Git tag.

*artifact list* [options...]
	List artifacts.

*artifact delete* <ID>
	Delete an artifact.

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

*ssh-key create* [path]
	Upload an SSH public key and associate it with your account.

	If _path_ is not specified, the default SSH public key is used.

*ssh-key delete* <ID>
	Delete an SSH public key from your account.

*pgp-key create* [path]
	Upload a PGP public key and associate it with your account.

	The public key must be in the armored format.

	If _path_ is not specified, the default public key from the local GPG
	keyring is used.

*pgp-key delete* <ID>
	Delete a PGP key from your account.

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
