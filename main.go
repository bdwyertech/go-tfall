package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/abiosoft/lineprefix"
	"github.com/hashicorp/go-multierror"
)

func main() {
	cmd := exec.Command("terraform", "workspace", "list")
	var b bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, io.MultiWriter(os.Stdout, &b), os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}

	var envs []string
	s := bufio.NewScanner(&b)
	for s.Scan() {
		l := strings.TrimSpace(strings.TrimPrefix(s.Text(), "*"))
		switch l {
		case "", "default":
			continue
		default:
			envs = append(envs, l)
		}
	}

	stdout, stderr := &lockedWriter{w: os.Stdout}, &lockedWriter{w: os.Stderr}

	var wg sync.WaitGroup
	cmdCtx := context.Background()
	var cmdErrs error
	for _, env := range envs {
		wg.Add(1)
		go func(env string) {
			cmd := exec.CommandContext(cmdCtx, "terraform", os.Args[1:]...)
			cmd.Stdout, cmd.Stderr = lineprefix.New(lineprefix.Prefix(env+": "), lineprefix.Writer(stdout)), stderr
			cmd.Env = append(os.Environ(), "TF_WORKSPACE="+env)
			if err := cmd.Run(); err != nil {
				cmdErrs = multierror.Append(cmdErrs, fmt.Errorf("%s: %w", env, err))
			}
			wg.Done()
		}(env)
	}
	wg.Wait()
	if cmdErrs != nil {
		log.Fatal(cmdErrs)
	}
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *lockedWriter) Write(b []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(b)
}
