package main

import (
	"errors"
	"io"
	"log"
	"os"
	"os/exec"

	"git.sr.ht/~xenrox/hut/termfmt"
	"github.com/google/shlex"
)

type pager interface {
	io.WriteCloser
	Running() bool
}

var pagerDone error = errors.New("paging is done")

func newPager() pager {
	if !termfmt.IsTerminal() {
		return &singleWritePager{os.Stdout, true}
	}

	name, ok := os.LookupEnv("PAGER")
	if !ok {
		name = "less"
	}

	commandSplit, err := shlex.Split(name)
	if err != nil {
		log.Fatalf("Failed to parse pager command: %v", err)
	}

	cmd := exec.Command(commandSplit[0], commandSplit[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "LESS=FRX")

	w, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to create stdin pipe for pager: %v", err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start pager %q: %v", cmd.Args[0], err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := cmd.Wait(); err != nil {
			log.Fatalf("Failed to run pager: %v", err)
		}
	}()

	return &cmdPager{w, done}
}

type pagerifyFn func(p pager) error

func pagerify(fn pagerifyFn) error {
	pager := newPager()
	defer pager.Close()

	for pager.Running() {
		err := fn(pager)
		if err == pagerDone {
			return nil
		} else if err != nil {
			return err
		}
	}

	return nil
}

type singleWritePager struct {
	io.WriteCloser
	running bool
}

func (p *singleWritePager) Write(b []byte) (int, error) {
	p.running = false
	return p.WriteCloser.Write(b)
}

func (p *singleWritePager) Running() bool {
	return p.running
}

type cmdPager struct {
	io.WriteCloser
	done <-chan struct{}
}

func (p *cmdPager) Close() error {
	if err := p.WriteCloser.Close(); err != nil {
		return err
	}
	<-p.done
	return nil
}

func (p *cmdPager) Running() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}
