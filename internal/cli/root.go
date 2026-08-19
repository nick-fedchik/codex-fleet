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
	workers.AddCommand(newWorkerAddCommand(opts, output, diagnostics))
	workers.AddCommand(newWorkerListCommand(opts, output))
	workers.AddCommand(newWorkerCheckCommand(opts, output, diagnostics))
	workers.AddCommand(newWorkerInspectCommand(opts, output, diagnostics))
	workers.AddCommand(newWorkerWarmupCommand(opts, output, diagnostics))
	workers.AddCommand(newWorkerRunCommand(opts, output, diagnostics))
	workers.AddCommand(newWorkerRemoveCommand(opts, output))
	return workers
}

func newWorkerAddCommand(opts *options, output, diagnostics io.Writer) *cobra.Command {
	var sshHost, sshUser, identity string
	var port int
	var verify bool
	command := &cobra.Command{
		Use:   "add NAME",
		Short: "Register or update a worker",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			if err != nil || !verify {
				return err
			}
			return inspectWorker(cmd.Context(), opts, output, diagnostics, args[0], "table", false)
		},
	}
	command.Flags().StringVar(&sshHost, "ssh-host", "", "SSH host or existing SSH alias")
	command.Flags().StringVar(&sshUser, "ssh-user", "", "optional SSH user override")
	command.Flags().IntVar(&port, "port", 0, "optional SSH port")
	command.Flags().StringVar(&identity, "identity", "", "optional SSH identity file")
	command.Flags().BoolVar(&verify, "check", false, "check the worker immediately after registration")
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
	var all bool
	command := &cobra.Command{
		Use:   "check [NAME]",
		Short: "Check whether a worker and Ollama are reachable",
		Args: func(_ *cobra.Command, args []string) error {
			if all && len(args) != 0 {
				return fmt.Errorf("--all cannot be combined with a worker name")
			}
			if !all && len(args) != 1 {
				return fmt.Errorf("provide NAME or use --all")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return checkAllWorkers(cmd.Context(), opts, output, diagnostics, format)
			}
			return inspectWorker(cmd.Context(), opts, output, diagnostics, args[0], format, false)
		},
	}
	command.Flags().StringVar(&format, "format", "table", "output format: table or json")
	command.Flags().BoolVar(&all, "all", false, "check every registered worker")
	return command
}

type workerCheckResult struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	Hostname string `json:"hostname,omitempty"`
	Models   int    `json:"models,omitempty"`
	Running  int    `json:"running,omitempty"`
	Error    string `json:"error,omitempty"`
}

func checkAllWorkers(ctx context.Context, opts *options, output, diagnostics io.Writer, format string) error {
	registry, err := loadRegistry(opts)
	if err != nil {
		return err
	}
	if format != "table" && format != "json" {
		return fmt.Errorf("unsupported format %q: use table or json", format)
	}
	results := make([]workerCheckResult, 0, len(registry.Workers))
	anyOffline := false
	for i := range registry.Workers {
		selected := &registry.Workers[i]
		inspection, checkErr := (worker.Transport{Timeout: 15 * time.Second}).Inspect(ctx, *selected)
		checkedAt := time.Now().UTC()
		selected.LastCheckedAt = &checkedAt
		result := workerCheckResult{Name: selected.Name}
		if checkErr != nil {
			selected.LastState = "offline"
			selected.LastError = checkErr.Error()
			result.State = "offline"
			result.Error = checkErr.Error()
			anyOffline = true
			_, _ = fmt.Fprintf(diagnostics, "%s: %s\n", selected.Name, checkErr)
		} else {
			selected.LastState = "online"
			selected.LastError = ""
			result.State = "online"
			result.Hostname = inspection.Hostname
			result.Models = len(inspection.Models)
			result.Running = len(inspection.Running)
		}
		results = append(results, result)
	}
	if path, pathErr := registryPath(opts); pathErr == nil {
		if saveErr := config.Save(path, registry); saveErr != nil {
			return saveErr
		}
	}
	if format == "json" {
		if err := writeJSON(output, results); err != nil {
			return err
		}
	} else {
		for _, result := range results {
			if result.State == "online" {
				if _, err := fmt.Fprintf(output, "online %s host=%s models=%d running=%d\n", result.Name, result.Hostname, result.Models, result.Running); err != nil {
					return err
				}
			} else if _, err := fmt.Fprintf(output, "offline %s error=%s\n", result.Name, result.Error); err != nil {
				return err
			}
		}
	}
	if anyOffline {
		return &exitError{code: 10, err: errors.New("one or more workers unavailable")}
	}
	return nil
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
	var model, prompt, keepAlive, format string
	var timeout time.Duration
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
			result, err := (worker.Transport{Timeout: timeout}).Run(cmd.Context(), *selected, model, prompt, keepAlive)
			if err != nil {
				_, _ = fmt.Fprintln(diagnostics, err)
				return &exitError{code: 13, err: errors.New("remote job failed")}
			}
			if format == "json" {
				return writeJSON(output, struct {
					Worker string                  `json:"worker"`
					Model  string                  `json:"model"`
					Result worker.GenerationResult `json:"result"`
				}{Worker: selected.Name, Model: model, Result: result})
			}
			if format != "text" {
				return fmt.Errorf("unsupported format %q: use text or json", format)
			}
			_, err = fmt.Fprintln(output, result.Response)
			return err
		},
	}
	command.Flags().StringVar(&model, "model", "", "Ollama model name")
	command.Flags().StringVar(&prompt, "prompt", "", "prompt text")
	command.Flags().StringVar(&keepAlive, "keep-alive", "10m", "how long Ollama keeps the model loaded")
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	command.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "maximum time to wait for SSH and Ollama")
	return command
}

func newWorkerWarmupCommand(opts *options, output, diagnostics io.Writer) *cobra.Command {
	var model, keepAlive, format string
	var timeout time.Duration
	command := &cobra.Command{
		Use:   "warmup NAME",
		Short: "Load a model and measure its loading time without a prompt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if model == "" {
				return fmt.Errorf("--model is required")
			}
			registry, err := loadRegistry(opts)
			if err != nil {
				return err
			}
			selected, err := registry.Find(args[0])
			if err != nil {
				return &exitError{code: 3, err: err}
			}
			result, err := (worker.Transport{Timeout: timeout}).Warmup(cmd.Context(), *selected, model, keepAlive)
			if err != nil {
				_, _ = fmt.Fprintln(diagnostics, err)
				return &exitError{code: 13, err: errors.New("worker warmup failed")}
			}
			if format == "json" {
				return writeJSON(output, struct {
					Worker    string                  `json:"worker"`
					Model     string                  `json:"model"`
					KeepAlive string                  `json:"keep_alive"`
					Result    worker.GenerationResult `json:"result"`
				}{Worker: selected.Name, Model: model, KeepAlive: keepAlive, Result: result})
			}
			if format != "table" {
				return fmt.Errorf("unsupported format %q: use table or json", format)
			}
			_, err = fmt.Fprintf(output, "warmed worker=%s model=%s keep_alive=%s load=%s total=%s\n", selected.Name, model, keepAlive, result.LoadDuration.Round(time.Millisecond), result.TotalDuration.Round(time.Millisecond))
			return err
		},
	}
	command.Flags().StringVar(&model, "model", "", "Ollama model name")
	command.Flags().StringVar(&keepAlive, "keep-alive", "10m", "how long Ollama keeps the model loaded")
	command.Flags().StringVar(&format, "format", "table", "output format: table or json")
	command.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "maximum time to wait for SSH and Ollama")
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
