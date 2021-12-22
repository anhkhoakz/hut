# hut

[![builds.sr.ht status](https://builds.sr.ht/~emersion/hut/commits.svg)](https://builds.sr.ht/~emersion/hut/commits?)

A CLI tool for sr.ht.

## Usage

1. [Generate](https://meta.sr.ht/oauth2/personal-token) a new OAuth2 access
   token.
2. Create a configuration file at `~/.config/hut/config`:

       instance "sr.ht" {
           access-token "<token>"
       }

3. `hut -h`

## Building

Dependencies:

- Go
- scdoc (optional, for man pages)

For end users, a `Makefile` is provided:

    make
    sudo make install

## License

AGPLv3, see LICENSE.

Copyright (C) 2021 Simon Ser
