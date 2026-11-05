package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/dansimau/hal"
	"github.com/dansimau/hal/hassws"
	"github.com/spf13/cobra"
)

func NewEventsCmd() *cobra.Command {
	var (
		excludes []string
		jqFilter string
	)

	cmd := &cobra.Command{
		Use:   "events",
		Short: "Stream raw Home Assistant events to stdout",
		Long: `Open a dedicated websocket connection to Home Assistant, subscribe to all
events, and print each one as formatted JSON as it arrives. Useful for live
debugging of automations.`,
		Example: `  hal events
  hal events --exclude state_changed
  hal events --exclude '*sun.sun*' --exclude '*device_tracker*'
  hal events --jq '.event | select(.event_type == "state_changed") | .data.entity_id'`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runEventsCommand(cmd.OutOrStdout(), excludes, jqFilter)
		},
	}

	cmd.Flags().StringArrayVar(&excludes, "exclude", nil,
		"Glob pattern; events whose JSON matches are suppressed (repeatable)")
	cmd.Flags().StringVar(&jqFilter, "jq", "", "Pipe each event JSON through this jq filter instead of pretty-printing")

	return cmd
}

func runEventsCommand(out io.Writer, excludes []string, jqFilter string) error {
	cfg, err := hal.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client := hassws.NewClient(hassws.ClientConfig{
		Host:         cfg.HomeAssistant.Host,
		Token:        cfg.HomeAssistant.Token,
		PingInterval: cfg.PingInterval,
		ReadTimeout:  cfg.ReadTimeout,
	})

	err = client.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	patterns, err := compileGlobs(excludes)
	if err != nil {
		return err
	}

	emit, cleanup, err := makeEmitter(out, jqFilter)
	if err != nil {
		return err
	}
	defer cleanup()

	err = client.SubscribeEventsRaw("", func(raw []byte) {
		for _, re := range patterns {
			if re.Match(raw) {
				return
			}
		}

		emit(raw)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	return nil
}

// makeEmitter returns a function that handles a single raw event payload.
// When jqFilter is empty, it pretty-prints the JSON to out. When set, it spawns
// a long-running `jq` subprocess writing to out and pipes each payload through it.
func makeEmitter(out io.Writer, jqFilter string) (emit func([]byte), cleanup func(), err error) {
	if jqFilter == "" {
		return func(raw []byte) {
			var pretty bytes.Buffer

			err := json.Indent(&pretty, raw, "", "  ")
			if err != nil {
				pretty.Write(raw)
			}

			fmt.Fprintln(out, pretty.String())
		}, func() {}, nil
	}

	cmd := exec.CommandContext(context.Background(), "jq", "-r", jqFilter)
	cmd.Stdout = out
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open jq stdin: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start jq (is it installed?): %w", err)
	}

	emit = func(raw []byte) {
		_, writeErr := stdin.Write(raw)
		if writeErr != nil {
			return
		}

		_, _ = io.WriteString(stdin, "\n")
	}

	cleanup = func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	}

	return emit, cleanup, nil
}

// compileGlobs converts each pattern (supporting `*` and `?`) into an
// unanchored regex that performs substring matching across the whole input,
// including newlines. Unanchored so callers can write `--exclude camera`
// instead of `--exclude '*camera*'`.
func compileGlobs(patterns []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(patterns))

	for _, pattern := range patterns {
		var builder strings.Builder

		builder.WriteString("(?s)")

		for _, r := range pattern {
			switch r {
			case '*':
				builder.WriteString(".*")
			case '?':
				builder.WriteString(".")
			default:
				builder.WriteString(regexp.QuoteMeta(string(r)))
			}
		}

		re, err := regexp.Compile(builder.String())
		if err != nil {
			return nil, fmt.Errorf("invalid exclude pattern %q: %w", pattern, err)
		}

		out = append(out, re)
	}

	return out, nil
}
