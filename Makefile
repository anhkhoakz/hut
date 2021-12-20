.POSIX:
.SUFFIXES:

GO = go
RM = rm
SCDOC = scdoc
GOFLAGS =
PREFIX = /usr/local
BINDIR = bin
MANDIR = share/man

all: hut doc/hut.1

hut:
	$(GO) build $(GOFLAGS)
doc/hut.1: doc/hut.1.scd
	$(SCDOC) <doc/hut.1.scd >doc/hut.1

clean:
	$(RM) -rf hut doc/hut.1
install:
	mkdir -p $(DESTDIR)$(PREFIX)/$(BINDIR)
	mkdir -p $(DESTDIR)$(PREFIX)/$(MANDIR)/man1
	cp -f hut $(DESTDIR)$(PREFIX)/$(BINDIR)
	cp -f doc/hut.1 $(DESTDIR)$(PREFIX)/$(MANDIR)/man1

.PHONY: hut
