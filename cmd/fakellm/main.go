// Command fakellm runs Sela's deterministic OpenAI-compatible TEST DOUBLE
// (internal/llm/fakellm) for tests, CI and local development. It is not a
// model and never Meta inference: it reports the model
// "test-double-not-meta" and the header "X-Sela-Test-Double: true" on every
// response.
//
// Usage:
//
//	fakellm [-addr 127.0.0.1:8090] [-api-key KEY] [-latency 0s] [-fault none] [-allow-fault-header]
//
// It never logs or prints request content.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/YanIanZ/sela/internal/llm/fakellm"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "fakellm:", err)
		os.Exit(1)
	}
}

// run parses args, serves until ctx is done and then shuts down gracefully.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("fakellm", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:8090", "listen address")
	apiKey := fs.String("api-key", "", "when set, require \"Authorization: Bearer <key>\" on /v1/*")
	latency := fs.Duration("latency", 0, "delay before each streamed chunk")
	faultName := fs.String("fault", string(fakellm.FaultNone), "fault for every chat completion: "+faultList())
	allowHeader := fs.Bool("allow-fault-header", false, "let requests choose a fault with the "+fakellm.HeaderFault+" header")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return errors.New("unexpected arguments")
	}
	fault, err := fakellm.ParseFault(*faultName)
	if err != nil {
		return fmt.Errorf("invalid -fault (want one of %s)", faultList())
	}
	handler, err := fakellm.NewHandler(fakellm.Config{
		APIKey: *apiKey, Latency: *latency, Fault: fault, AllowFaultHeader: *allowHeader,
	})
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	auth := "none"
	if *apiKey != "" {
		auth = "bearer"
	}
	fmt.Fprintf(stdout, "fakellm: TEST DOUBLE (not Meta inference) model=%s fault=%s auth=%s listening on http://%s\n",
		fakellm.ServedModel, fault, auth, ln.Addr())

	srv := &http.Server{
		Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute,
		// Request contexts end with ctx, so stalled or slow fault responses
		// stop as soon as shutdown begins.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil {
		// Stalled streams wait for their client; close them.
		_ = srv.Close()
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func faultList() string {
	names := make([]string, 0, len(fakellm.Faults()))
	for _, f := range fakellm.Faults() {
		names = append(names, string(f))
	}
	return strings.Join(names, ", ")
}
