# hut

A CLI tool for sr.ht.

## Usage

1. [Generate](https://meta.sr.ht/oauth2/personal-token) a new OAuth2 access
   token.
2. Create a configuration file at `~/.config/hut/config`:

       instance "sr.ht" {
           access-token "<token>"
       }

3. `hut -h`

## License

GPLv3, see LICENSE.

Copyright (C) 2020 Simon Ser
