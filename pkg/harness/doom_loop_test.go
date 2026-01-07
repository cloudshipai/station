package harness

import (
	"testing"
)

func TestDoomLoopDetector_NewDetector(t *testing.T) {
	detector := NewDoomLoopDetector(3)
	if detector.threshold != 3 {
		t.Errorf("threshold = %d, want %d", detector.threshold, 3)
	}

	detectorDefault := NewDoomLoopDetector(0)
	if detectorDefault.threshold != 3 {
		t.Errorf("default threshold = %d, want %d", detectorDefault.threshold, 3)
	}
}

func TestDoomLoopDetector_DetectsLoop(t *testing.T) {
	detector := NewDoomLoopDetector(3)

	args := map[string]interface{}{"file": "test.txt"}

	if detector.IsInDoomLoop("read_file", args) {
		t.Error("Should not detect loop on first call")
	}
	detector.Record("read_file", args)

	if detector.IsInDoomLoop("read_file", args) {
		t.Error("Should not detect loop on second call")
	}
	detector.Record("read_file", args)

	if !detector.IsInDoomLoop("read_file", args) {
		t.Error("Should detect loop on third consecutive call")
	}
}

func TestDoomLoopDetector_DifferentArgs(t *testing.T) {
	detector := NewDoomLoopDetector(3)

	args1 := map[string]interface{}{"file": "test1.txt"}
	args2 := map[string]interface{}{"file": "test2.txt"}

	detector.IsInDoomLoop("read_file", args1)
	detector.Record("read_file", args1)

	detector.IsInDoomLoop("read_file", args2)
	detector.Record("read_file", args2)

	detector.IsInDoomLoop("read_file", args1)
	detector.Record("read_file", args1)

	if detector.IsInDoomLoop("read_file", args2) {
		t.Error("Should not detect loop with different args interleaved")
	}
}

func TestDoomLoopDetector_DifferentTools(t *testing.T) {
	detector := NewDoomLoopDetector(3)

	args := map[string]interface{}{"path": "/"}

	detector.IsInDoomLoop("list_files", args)
	detector.Record("list_files", args)

	detector.IsInDoomLoop("read_file", args)
	detector.Record("read_file", args)

	detector.IsInDoomLoop("list_files", args)
	detector.Record("list_files", args)

	if detector.IsInDoomLoop("read_file", args) {
		t.Error("Should not detect loop with different tools interleaved")
	}
}

func TestDoomLoopDetector_Reset(t *testing.T) {
	detector := NewDoomLoopDetector(3)

	args := map[string]interface{}{"file": "test.txt"}

	detector.IsInDoomLoop("read_file", args)
	detector.Record("read_file", args)
	detector.IsInDoomLoop("read_file", args)
	detector.Record("read_file", args)

	detector.Reset()

	if detector.IsInDoomLoop("read_file", args) {
		t.Error("Should not detect loop after reset")
	}
}

func TestDoomLoopDetector_GetConsecutiveCount(t *testing.T) {
	detector := NewDoomLoopDetector(5)

	args := map[string]interface{}{"cmd": "ls"}

	detector.IsInDoomLoop("bash", args)
	detector.Record("bash", args)
	detector.IsInDoomLoop("bash", args)
	detector.Record("bash", args)
	detector.IsInDoomLoop("bash", args)

	count := detector.GetConsecutiveCount("bash", args)
	if count != 3 {
		t.Errorf("consecutive count = %d, want %d", count, 3)
	}
}

func TestHashArgs(t *testing.T) {
	tests := []struct {
		name   string
		args   interface{}
		stable bool
	}{
		{"nil", nil, true},
		{"empty map", map[string]interface{}{}, true},
		{"simple map", map[string]interface{}{"key": "value"}, true},
		{"nested map", map[string]interface{}{"outer": map[string]interface{}{"inner": "value"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := hashArgs(tt.args)
			hash2 := hashArgs(tt.args)

			if tt.stable && hash1 != hash2 {
				t.Errorf("hash not stable for %v: %s != %s", tt.args, hash1, hash2)
			}

			if hash1 == "" {
				t.Error("hash should not be empty")
			}
		})
	}

	hash1 := hashArgs(map[string]interface{}{"a": 1})
	hash2 := hashArgs(map[string]interface{}{"b": 2})
	if hash1 == hash2 {
		t.Error("different args should have different hashes")
	}
}
