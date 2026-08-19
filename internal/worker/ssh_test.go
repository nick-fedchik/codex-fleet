package worker

import "testing"

func TestParseInspection(t *testing.T) {
	input := `__CF_HOSTNAME__jetson
__CF_USER__codex
__CF_TAGS__
{"models":[{"name":"qwen3:4b","model":"qwen3:4b","size":123}]}
__CF_TAGS_END__
__CF_PS__
{"models":[{"name":"qwen3:4b","model":"qwen3:4b","size_vram":456}]}
__CF_PS_END__
`
	inspection, err := parseInspection("jetson", []byte(input))
	if err != nil {
		t.Fatalf("parseInspection() error = %v", err)
	}
	if inspection.Hostname != "jetson" || inspection.User != "codex" {
		t.Fatalf("identity = %#v", inspection)
	}
	if len(inspection.Models) != 1 || inspection.Models[0].Name != "qwen3:4b" {
		t.Fatalf("models = %#v", inspection.Models)
	}
	if len(inspection.Running) != 1 || inspection.Running[0].SizeVRAM != 456 {
		t.Fatalf("running = %#v", inspection.Running)
	}
}
