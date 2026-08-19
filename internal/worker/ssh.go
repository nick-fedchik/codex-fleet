package worker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/nick-fedchik/codex-fleet/internal/config"
)

type Model struct {
	Name       string `json:"name"`
	Model      string `json:"model"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
	ModifiedAt string `json:"modified_at"`
}

type Runtime struct {
	Name       string `json:"name"`
	Model      string `json:"model"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
	ExpiresAt  string `json:"expires_at"`
	SizeVRAM   int64  `json:"size_vram"`
	SizeLoaded int64  `json:"size_loaded"`
}

type Inspection struct {
	Worker         string        `json:"worker"`
	Hostname       string        `json:"hostname"`
	User           string        `json:"user"`
	Latency        time.Duration `json:"latency_ns"`
	Models         []Model       `json:"models"`
	Running        []Runtime     `json:"running"`
	OllamaEndpoint string        `json:"ollama_endpoint"`
}

type Transport struct {
	Timeout time.Duration
}

func (t Transport) Check(ctx context.Context, worker config.Worker) (Inspection, error) {
	return t.inspect(ctx, worker)
}

func (t Transport) Inspect(ctx context.Context, worker config.Worker) (Inspection, error) {
	return t.inspect(ctx, worker)
}

func (t Transport) Run(ctx context.Context, worker config.Worker, model, prompt string) (string, error) {
	payload, err := json.Marshal(struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		Stream bool   `json:"stream"`
	}{Model: model, Prompt: prompt, Stream: false})
	if err != nil {
		return "", fmt.Errorf("encode prompt: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(payload)
	remote := fmt.Sprintf("printf '%%s' '%s' | base64 -d | curl -fsS --max-time 600 -H 'Content-Type: application/json' -d @- http://127.0.0.1:11434/api/generate", encoded)
	output, err := t.run(ctx, worker, remote, nil)
	if err != nil {
		return "", err
	}
	var response struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return "", fmt.Errorf("parse Ollama response: %w", err)
	}
	if response.Error != "" {
		return "", fmt.Errorf("Ollama: %s", response.Error)
	}
	return response.Response, nil
}

func (t Transport) inspect(ctx context.Context, worker config.Worker) (Inspection, error) {
	started := time.Now()
	remote := "printf '__CF_HOSTNAME__%s\\n' \"$(hostname)\"; printf '__CF_USER__%s\\n' \"$(id -un)\"; printf '__CF_TAGS__\\n'; curl -fsS --max-time 5 http://127.0.0.1:11434/api/tags; printf '\\n__CF_TAGS_END__\\n'; printf '__CF_PS__\\n'; curl -fsS --max-time 5 http://127.0.0.1:11434/api/ps; printf '\\n__CF_PS_END__\\n'"
	output, err := t.run(ctx, worker, remote, nil)
	if err != nil {
		return Inspection{}, err
	}
	inspection, err := parseInspection(worker.Name, output)
	if err != nil {
		return Inspection{}, err
	}
	inspection.Latency = time.Since(started)
	inspection.OllamaEndpoint = "http://127.0.0.1:11434"
	return inspection, nil
}

func (t Transport) run(ctx context.Context, worker config.Worker, remote string, stdin []byte) ([]byte, error) {
	timeout := t.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{"-T", "-o", "BatchMode=yes", "-o", "RequestTTY=no", "-o", "ConnectTimeout=5"}
	if worker.Port > 0 {
		args = append(args, "-p", strconv.Itoa(worker.Port))
	}
	if worker.IdentityFile != "" {
		args = append(args, "-i", worker.IdentityFile)
	}
	target := worker.SSHHost
	if worker.SSHUser != "" {
		target = worker.SSHUser + "@" + target
	}
	args = append(args, target, remote)
	command := exec.CommandContext(commandCtx, "ssh", args...)
	if stdin != nil {
		command.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if commandCtx.Err() != nil {
			return nil, fmt.Errorf("worker unavailable or timeout: %w", commandCtx.Err())
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("SSH transport: %s", message)
	}
	return stdout.Bytes(), nil
}

func parseInspection(name string, output []byte) (Inspection, error) {
	text := string(output)
	hostname, ok := marker(text, "__CF_HOSTNAME__", "\n")
	if !ok {
		return Inspection{}, fmt.Errorf("parse remote hostname")
	}
	user, ok := marker(text, "__CF_USER__", "\n")
	if !ok {
		return Inspection{}, fmt.Errorf("parse remote user")
	}
	tagsText, ok := section(text, "__CF_TAGS__\n", "\n__CF_TAGS_END__")
	if !ok {
		return Inspection{}, fmt.Errorf("parse Ollama model response")
	}
	psText, ok := section(text, "__CF_PS__\n", "\n__CF_PS_END__")
	if !ok {
		return Inspection{}, fmt.Errorf("parse Ollama process response")
	}
	var tags struct {
		Models []Model `json:"models"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(tagsText)), &tags); err != nil {
		return Inspection{}, fmt.Errorf("parse Ollama models: %w", err)
	}
	var ps struct {
		Models []Runtime `json:"models"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(psText)), &ps); err != nil {
		return Inspection{}, fmt.Errorf("parse Ollama running models: %w", err)
	}
	return Inspection{Worker: name, Hostname: hostname, User: user, Models: tags.Models, Running: ps.Models}, nil
}

func marker(text, start, end string) (string, bool) {
	index := strings.Index(text, start)
	if index < 0 {
		return "", false
	}
	value := text[index+len(start):]
	if endIndex := strings.Index(value, end); endIndex >= 0 {
		return strings.TrimSpace(value[:endIndex]), true
	}
	return "", false
}

func section(text, start, end string) (string, bool) {
	index := strings.Index(text, start)
	if index < 0 {
		return "", false
	}
	value := text[index+len(start):]
	endIndex := strings.Index(value, end)
	if endIndex < 0 {
		return "", false
	}
	return value[:endIndex], true
}
