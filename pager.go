package main

import (
	"io"
	"log"
	"os"
	"os/exec"

	"git.sr.ht/~emersion/hut/termfmt"
)

type pager interface {
	io.WriteCloser
	Running() bool
}

func newPager() pager {
	if !termfmt.IsTerminal() {
		return &singleWritePager{os.Stdout, true}
	}

	cmd := exec.Command("less")
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

type pagerifyFn func(p pager) bool

func pagerify(fn pagerifyFn) {
	pager := newPager()
	defer pager.Close()

	for pager.Running() {
		shouldStop := fn(pager)
		if shouldStop {
			break
		}
	}
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
