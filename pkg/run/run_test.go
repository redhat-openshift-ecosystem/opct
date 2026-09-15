package run

import "testing"

func TestUnsupportedSkipChecksAlias(t *testing.T) {
	cmd := NewCmdRun()
	if cmd.Flags().Lookup("unsupported-skip-checks") == nil {
		t.Fatal("unsupported-skip-checks flag is not registered")
	}
	if err := cmd.Flags().Set("unsupported-skip-checks", "true"); err != nil {
		t.Fatalf("setting unsupported-skip-checks: %v", err)
	}

	got, err := cmd.Flags().GetBool("devel-skip-checks")
	if err != nil {
		t.Fatalf("getting devel-skip-checks: %v", err)
	}
	if !got {
		t.Error("unsupported-skip-checks did not set devel-skip-checks")
	}
}
