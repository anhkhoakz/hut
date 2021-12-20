hut(1)

# NAME

hut - A CLI tool for sr.ht

# SYNOPSIS

*hut* [commands...] [options...]

# DESCRIPTION

hut is a CLI companion utility to interact with sr.ht.

# OPTIONS

*-h, --help*
	Show help message and quit.
	Can be used after a command to get more information it.

# COMMANDS

*artifact* <filenames> [options...]
	Upload an artifact.

	Options are:

	*-r, --repo* <string>
		Name of repository.

	*--rev* <string>
		Revision tag.

*build* <manifest> [options...]
	Submit a build manifest.

	Options are:

	*-f,  --follow*
		Follow build logs.

*help* <command>
	Help about any command.

*paste* <filenames>
	Create a new paste.

# CONFIGURATION

Generate a new OAuth2 access token on _meta.sr.ht_.

On startup hut will look for a file at *$XDG_CONFIG_HOME/hut/config*. If
unset, $XDG_CONFIG_HOME defaults to *~/.config/*.

```
instance "sr.ht" {
    access-token "<token>"
}
```

# AUTHORS

Maintained by Simon Ser <contact@emersion.fr>, who is assisted by other
open-source contributors. For more information about hut development, see
<https://sr.ht/~emersion/hut>.

