package config

import "testing"

func TestRegistrySaveLoadAndRemove(t *testing.T) {
	path := t.TempDir() + "/workers.json"
	want := Registry{Workers: []Worker{{Name: "worker-1", SSHHost: "worker-host", SSHUser: "fleet-user"}}}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Workers) != 1 || got.Workers[0].Name != "worker-1" {
		t.Fatalf("Load() = %#v, want one worker-1 worker", got)
	}
	if err := got.Remove("worker-1"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := got.Find("worker-1"); err == nil {
		t.Fatal("Find() unexpectedly found removed worker")
	}
}
