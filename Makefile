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

# Cross-compilation targets
PLATFORMS = linux/amd64 linux/arm64 linux/386 windows/amd64 windows/386 darwin/amd64 darwin/arm64 freebsd/amd64 freebsd/arm64 openbsd/amd64 openbsd/arm64 netbsd/amd64 netbsd/arm64

all: hut completions doc/hut.1

hut:
	$(GO) build $(GOFLAGS)

# Cross-compilation targets
cross: $(addprefix hut-,$(subst /,-,$(PLATFORMS)))

hut-%:
	@echo "Building for $(subst -,/,$*)" && \
	GOOS=$(word 1,$(subst /, ,$*)) GOARCH=$(word 2,$(subst /, ,$*)) $(GO) build $(GOFLAGS) -o hut-$(subst /,-,$*) .

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
	$(RM) -f hut doc/hut.1 hut.bash hut.zsh hut.fish hut-*

install:
	$(INSTALL) -d \
		$(DESTDIR)$(PREFIX)/$(BINDIR)/ \
		$(DESTDIR)$(PREFIX)/$(MANDIR)/man1/ \
		$(DESTDIR)$(BASHCOMPDIR) \
		$(DESTDIR)$(ZSHCOMPDIR) \
		$(DESTDIR)$(FISHCOMPDIR)
	$(INSTALL) -pm 0755 hut $(DESTDIR)$(PREFIX)/$(BINDIR)/
	$(INSTALL) -pm 0644 doc/hut.1 $(DESTDIR)$(PREFIX)/$(MANDIR)/man1/
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

.PHONY: all hut cross clean install uninstall completions
