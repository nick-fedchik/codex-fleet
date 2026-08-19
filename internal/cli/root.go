package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nick-fedchik/codex-fleet/internal/config"
	"github.com/nick-fedchik/codex-fleet/internal/worker"
	"github.com/spf13/cobra"
)

const version = "0.1.0-dev"

type options struct {
	configPath string
}

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func Execute() {
	if err := NewRoot(os.Stdout, os.Stderr).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCode(err))
	}
}

func NewRoot(output, diagnostics io.Writer) *cobra.Command {
	var opts options
	root := &cobra.Command{
		Use:           "codex-fleet",
		Short:         "Manage a deterministic fleet of local Ollama workers",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(output)
	root.SetErr(diagnostics)
	root.PersistentFlags().StringVar(&opts.configPath, "config", "", "worker registry path")
	root.AddCommand(newWorkerCommand(&opts, output, diagnostics))
	root.AddCommand(newConfigCommand(&opts, output))
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version)
		},
	})
	return root
}

func newConfigCommand(opts *options, output io.Writer) *cobra.Command {
	configCommand := &cobra.Command{Use: "config", Short: "Inspect local configuration", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() }}
	configCommand.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print the active worker registry path",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			path, err := registryPath(opts)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(output, path)
			return err
		},
	})
	return configCommand
}

func newWorkerCommand(opts *options, output, diagnostics io.Writer) *cobra.Command {
	workers := &cobra.Command{Use: "worker", Short: "Register and operate Ollama workers", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() }}
	workers.AddCommand(newWorkerAddCommand(opts, output))
	workers.AddCommand(newWorkerListCommand(opts, output))
	workers.AddCommand(newWorkerCheckCommand(opts, output, diagnostics))
	workers.AddCommand(newWorkerInspectCommand(opts, output, diagnostics))
	workers.AddCommand(newWorkerRunCommand(opts, output, diagnostics))
	workers.AddCommand(newWorkerRemoveCommand(opts, output))
	return workers
}

func newWorkerAddCommand(opts *options, output io.Writer) *cobra.Command {
	var sshHost, sshUser, identity string
	var port int
	command := &cobra.Command{
		Use:   "add NAME",
		Short: "Register or update a worker",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if strings.TrimSpace(sshHost) == "" {
				return fmt.Errorf("--ssh-host is required")
			}
			path, err := registryPath(opts)
			if err != nil {
				return err
			}
			registry, err := config.Load(path)
			if err != nil {
				return err
			}
			registry.Upsert(config.Worker{Name: args[0], SSHHost: sshHost, SSHUser: sshUser, Port: port, IdentityFile: identity})
			if err := config.Save(path, registry); err != nil {
				return err
			}
			_, err = fmt.Fprintf(output, "registered worker %s\n", args[0])
			return err
		},
	}
	command.Flags().StringVar(&sshHost, "ssh-host", "", "SSH host or existing SSH alias")
	command.Flags().StringVar(&sshUser, "ssh-user", "", "optional SSH user override")
	command.Flags().IntVar(&port, "port", 0, "optional SSH port")
	command.Flags().StringVar(&identity, "identity", "", "optional SSH identity file")
	return command
}

func newWorkerListCommand(opts *options, output io.Writer) *cobra.Command {
	var format string
	command := &cobra.Command{
		Use:   "list",
		Short: "List registered workers and their last known state",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			registry, err := loadRegistry(opts)
			if err != nil {
				return err
			}
			if format == "json" {
				return writeJSON(output, registry.Workers)
			}
			if format != "table" {
				return fmt.Errorf("unsupported format %q: use table or json", format)
			}
			if len(registry.Workers) == 0 {
				_, err := fmt.Fprintln(output, "no workers registered")
				return err
			}
			if _, err := fmt.Fprintln(output, "NAME\tSSH HOST\tSTATE\tLAST CHECK"); err != nil {
				return err
			}
			for _, worker := range registry.Workers {
				lastCheck := "never"
				if worker.LastCheckedAt != nil {
					lastCheck = worker.LastCheckedAt.Format(time.RFC3339)
				}
				state := worker.LastState
				if state == "" {
					state = "unknown"
				}
				if _, err := fmt.Fprintf(output, "%s\t%s\t%s\t%s\n", worker.Name, worker.SSHHost, state, lastCheck); err != nil {
					return err
				}
			}
			return nil
		},
	}
	command.Flags().StringVar(&format, "format", "table", "output format: table or json")
	return command
}

func newWorkerCheckCommand(opts *options, output, diagnostics io.Writer) *cobra.Command {
	var format string
	command := &cobra.Command{
		Use:   "check NAME",
		Short: "Check whether a worker and Ollama are reachable",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return inspectWorker(cmd.Context(), opts, output, diagnostics, args[0], format, false)
		},
	}
	command.Flags().StringVar(&format, "format", "table", "output format: table or json")
	return command
}

func newWorkerInspectCommand(opts *options, output, diagnostics io.Writer) *cobra.Command {
	var format string
	command := &cobra.Command{
		Use:   "inspect NAME",
		Short: "Fetch live worker identity, models, and runtime details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return inspectWorker(cmd.Context(), opts, output, diagnostics, args[0], format, true)
		},
	}
	command.Flags().StringVar(&format, "format", "table", "output format: table or json")
	return command
}

func newWorkerRunCommand(opts *options, output, diagnostics io.Writer) *cobra.Command {
	var model, prompt string
	command := &cobra.Command{
		Use:   "run NAME",
		Short: "Run one explicit prompt on a worker",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if model == "" {
				return fmt.Errorf("--model is required")
			}
			if prompt == "" {
				return fmt.Errorf("--prompt is required")
			}
			registry, err := loadRegistry(opts)
			if err != nil {
				return err
			}
			selected, err := registry.Find(args[0])
			if err != nil {
				return &exitError{code: 3, err: err}
			}
			result, err := (worker.Transport{Timeout: 10 * time.Minute}).Run(cmd.Context(), *selected, model, prompt)
			if err != nil {
				_, _ = fmt.Fprintln(diagnostics, err)
				return &exitError{code: 13, err: errors.New("remote job failed")}
			}
			_, err = fmt.Fprintln(output, result)
			return err
		},
	}
	command.Flags().StringVar(&model, "model", "", "Ollama model name")
	command.Flags().StringVar(&prompt, "prompt", "", "prompt text")
	return command
}

func newWorkerRemoveCommand(opts *options, output io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "remove NAME",
		Short: "Remove a worker from the local registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			path, err := registryPath(opts)
			if err != nil {
				return err
			}
			registry, err := config.Load(path)
			if err != nil {
				return err
			}
			if err := registry.Remove(args[0]); err != nil {
				return &exitError{code: 3, err: err}
			}
			if err := config.Save(path, registry); err != nil {
				return err
			}
			_, err = fmt.Fprintf(output, "removed worker %s\n", args[0])
			return err
		},
	}
	return command
}

func inspectWorker(ctx context.Context, opts *options, output, diagnostics io.Writer, name, format string, detailed bool) error {
	registry, err := loadRegistry(opts)
	if err != nil {
		return err
	}
	selected, err := registry.Find(name)
	if err != nil {
		return &exitError{code: 3, err: err}
	}
	inspection, checkErr := (worker.Transport{Timeout: 15 * time.Second}).Inspect(ctx, *selected)
	checkedAt := time.Now().UTC()
	selected.LastCheckedAt = &checkedAt
	if checkErr != nil {
		selected.LastState = "offline"
		selected.LastError = checkErr.Error()
	} else {
		selected.LastState = "online"
		selected.LastError = ""
	}
	if path, pathErr := registryPath(opts); pathErr == nil {
		_ = config.Save(path, registry)
	}
	if checkErr != nil {
		_, _ = fmt.Fprintln(diagnostics, checkErr)
		return &exitError{code: 10, err: errors.New("worker unavailable")}
	}
	if format == "json" {
		return writeJSON(output, inspection)
	}
	if format != "table" {
		return fmt.Errorf("unsupported format %q: use table or json", format)
	}
	if !detailed {
		_, err = fmt.Fprintf(output, "online %s host=%s user=%s models=%d running=%d latency=%s\n", inspection.Worker, inspection.Hostname, inspection.User, len(inspection.Models), len(inspection.Running), inspection.Latency.Round(time.Millisecond))
		return err
	}
	_, err = fmt.Fprintf(output, "worker: %s\nhostname: %s\nuser: %s\nlatency: %s\nmodels:\n", inspection.Worker, inspection.Hostname, inspection.User, inspection.Latency.Round(time.Millisecond))
	if err != nil {
		return err
	}
	for _, model := range inspection.Models {
		if _, err := fmt.Fprintf(output, "  - %s\n", model.Name); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(output, "running:")
	for _, running := range inspection.Running {
		if _, err := fmt.Fprintf(output, "  - %s\n", running.Name); err != nil {
			return err
		}
	}
	return err
}

func loadRegistry(opts *options) (config.Registry, error) {
	path, err := registryPath(opts)
	if err != nil {
		return config.Registry{}, err
	}
	return config.Load(path)
}

func registryPath(opts *options) (string, error) {
	if opts.configPath != "" {
		return opts.configPath, nil
	}
	return config.DefaultPath()
}

func writeJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func exitCode(err error) int {
	var coded *exitError
	if errors.As(err, &coded) {
		return coded.code
	}
	return 1
}
