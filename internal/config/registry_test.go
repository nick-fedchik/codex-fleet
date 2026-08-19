package config

import "testing"

func TestRegistrySaveLoadAndRemove(t *testing.T) {
	path := t.TempDir() + "/workers.json"
	want := Registry{Workers: []Worker{{Name: "jetson", SSHHost: "jetson-codex", SSHUser: "codex"}}}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Workers) != 1 || got.Workers[0].Name != "jetson" {
		t.Fatalf("Load() = %#v, want one jetson worker", got)
	}
	if err := got.Remove("jetson"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := got.Find("jetson"); err == nil {
		t.Fatal("Find() unexpectedly found removed worker")
	}
}
