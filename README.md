# [hut]

[![builds.sr.ht status](https://builds.sr.ht/~emersion/hut/commits.svg)](https://builds.sr.ht/~emersion/hut/commits?)

A CLI tool for sr.ht.

## Usage

1. [Generate](https://meta.sr.ht/oauth2/personal-token) a new OAuth2 access
   token.
2. Create a configuration file at `~/.config/hut/config`:

       instance "sr.ht" {
           access-token "<token>"
       }

3. `man hut`

## Building

Dependencies:

- Go
- scdoc (optional, for man pages)

For end users, a `Makefile` is provided:

    make
    sudo make install

## Contributing

Send patches to the [mailing list], report bugs on the [issue tracker].

## License

AGPLv3, see [LICENSE].

Copyright (C) 2021 Simon Ser

[hut]: https://sr.ht/~emersion/hut/
[mailing list]: https://lists.sr.ht/~emersion/hut-dev
[issue tracker]: https://todo.sr.ht/~emersion/hut
[LICENSE]: LICENSE
