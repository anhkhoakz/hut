.POSIX:
.SUFFIXES:

GO = go
RM = rm
INSTALL = install
SCDOC = scdoc
GOFLAGS =
PREFIX = /usr/local
BINDIR = bin
MANDIR = share/man
BASHCOMPDIR = $(PREFIX)/share/bash-completion/completions
ZSHCOMPDIR = $(PREFIX)/share/zsh/site-functions
FISHCOMPDIR = $(PREFIX)/share/fish/vendor_completions.d

all: hut completions doc/hut.1

hut:
	$(GO) build $(GOFLAGS)

completions: hut.bash hut.zsh hut.fish

hut.bash: hut
	./hut completion bash >hut.bash

hut.zsh: hut
	./hut completion zsh >hut.zsh

hut.fish: hut
	./hut completion fish >hut.fish

doc/hut.1: doc/hut.1.scd
	$(SCDOC) <doc/hut.1.scd >doc/hut.1

clean:
	$(RM) -f hut doc/hut.1 hut.bash hut.zsh hut.fish

install:
	$(INSTALL) -dp \
		$(DESTDIR)$(PREFIX)/$(BINDIR)/ \
		$(DESTDIR)$(PREFIX)/$(MANDIR)/man1/ \
		$(DESTDIR)$(BASHCOMPDIR) \
		$(DESTDIR)$(ZSHCOMPDIR) \
		$(DESTDIR)$(FISHCOMPDIR)
	$(INSTALL) -pm 0755 hut -t $(DESTDIR)$(PREFIX)/$(BINDIR)/
	$(INSTALL) -pm 0644 doc/hut.1 -t $(DESTDIR)$(PREFIX)/$(MANDIR)/man1/
	$(INSTALL) -pm 0644 hut.bash $(DESTDIR)$(BASHCOMPDIR)/hut
	$(INSTALL) -pm 0644 hut.zsh $(DESTDIR)$(ZSHCOMPDIR)/_hut
	$(INSTALL) -pm 0644 hut.fish $(DESTDIR)$(FISHCOMPDIR)/hut.fish

uninstall:
	$(RM) -f \
		$(DESTDIR)$(PREFIX)/$(BINDIR)/hut \
		$(DESTDIR)$(PREFIX)/$(MANDIR)/man1/hut.1 \
		$(DESTDIR)$(BASHCOMPDIR)/hut \
		$(DESTDIR)$(ZSHCOMPDIR)/_hut \
		$(DESTDIR)$(FISHCOMPDIR)/hut.fish

.PHONY: all hut clean install uninstall completions
