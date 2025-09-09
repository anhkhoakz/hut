# [hut]

[![builds.sr.ht status](https://builds.xenrox.net/~xenrox/hut/commits/master.svg)](https://builds.xenrox.net/~xenrox/hut/commits/master?)

A CLI tool for [sr.ht].

## Usage

Run `hut init` to get started. Read the [man page] to learn about all commands.

## Building

Dependencies:

- Go
- scdoc (optional, for man pages)

For end users, a `Makefile` is provided:

    make
    sudo make install

### Cross-compilation

The project supports cross-compilation for multiple platforms:

**Using Makefile:**
```bash
# Build for all supported platforms
make cross

# Build for specific platform (e.g., Windows AMD64)
make hut-windows-amd64
```

**Using build script:**
```bash
# Build for all supported platforms
./build.sh

# Build for specific platforms
./build.sh linux/amd64 windows/amd64 darwin/arm64
```

**Supported platforms:**
- Linux: amd64, arm64, 386
- Windows: amd64, 386
- macOS: amd64, arm64
- FreeBSD: amd64, arm64
- OpenBSD: amd64, arm64
- NetBSD: amd64, arm64

Built binaries will be placed in the `build/` directory with appropriate file extensions (.exe for Windows).

## Contributing

Send patches to the [mailing list], report bugs on the [issue tracker].

Join the IRC channel: [#hut on Libera Chat].

## License

AGPLv3 only, see [LICENSE].

Copyright (C) 2021 Simon Ser

[#hut on Libera Chat]: ircs://irc.libera.chat/#hut
[hut]: https://sr.ht/~xenrox/hut/
[issue tracker]: https://todo.sr.ht/~xenrox/hut
[LICENSE]: LICENSE
[mailing list]: https://lists.sr.ht/~xenrox/hut-dev
[man page]: https://git.sr.ht/~xenrox/hut/tree/master/item/doc/hut.1.scd
[sr.ht]: https://sr.ht/~sircmpwn/sourcehut/
