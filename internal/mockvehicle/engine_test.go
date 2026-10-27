package mockvehicle

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMockVehicle_ClimateControl(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "mock_test.log")

	engine, err := NewEngine(logPath)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// 1. Valid temp 24.5
	res, err := engine.Execute("climate.set_temperature", map[string]interface{}{
		"target_temperature": 24.5,
		"zone":               "driver",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !res.Accepted || res.Entry.Decision != DecisionAccepted {
		t.Errorf("expected accepted, got %v", res.Entry.Decision)
	}
	if res.Entry.Reason != ReasonValidSlotsSafeRange {
		t.Errorf("expected %s, got %s", ReasonValidSlotsSafeRange, res.Entry.Reason)
	}
	if !res.Entry.Simulation {
		t.Errorf("simulation must be true")
	}

	// 2. Out of range temp 35.0
	resOut, err := engine.Execute("climate.set_temperature", map[string]interface{}{
		"target_temperature": 35.0,
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if resOut.Accepted || resOut.Entry.Decision != DecisionRejected {
		t.Errorf("expected rejected for temp 35, got %v", resOut.Entry.Decision)
	}
	if resOut.Entry.Reason != ReasonTempOutOfRange {
		t.Errorf("expected %s, got %s", ReasonTempOutOfRange, resOut.Entry.Reason)
	}

	// 3. Missing temperature slot
	resMissing, err := engine.Execute("climate.set_temperature", map[string]interface{}{
		"zone": "all",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if resMissing.Accepted || resMissing.Entry.Decision != DecisionRejected {
		t.Errorf("expected rejected for missing temp, got %v", resMissing.Entry.Decision)
	}
	if resMissing.Entry.Reason != ReasonSlotMissing {
		t.Errorf("expected %s, got %s", ReasonSlotMissing, resMissing.Entry.Reason)
	}
}

func TestMockVehicle_WindowControl(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "mock_window.log")

	engine, err := NewEngine(logPath)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Valid window open
	res, err := engine.Execute("window.control", map[string]interface{}{
		"action":   "open",
		"position": "driver",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !res.Accepted || res.Entry.Decision != DecisionAccepted {
		t.Errorf("expected accepted, got %v", res.Entry.Decision)
	}

	// Invalid action
	resInv, err := engine.Execute("window.control", map[string]interface{}{
		"action": "break_glass",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if resInv.Accepted || resInv.Entry.Decision != DecisionRejected {
		t.Errorf("expected rejected, got %v", resInv.Entry.Decision)
	}
	if resInv.Entry.Reason != ReasonUnsupportedWindowAction {
		t.Errorf("expected %s, got %s", ReasonUnsupportedWindowAction, resInv.Entry.Reason)
	}
}

func TestMockVehicle_DriveModeDangerousRejection(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "mock_drive.log")

	engine, err := NewEngine(logPath)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Valid sport mode
	res, err := engine.Execute("drive.switch_mode", map[string]interface{}{
		"mode": "sport",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !res.Accepted || res.Entry.Decision != DecisionAccepted {
		t.Errorf("expected accepted for sport mode, got %v", res.Entry.Decision)
	}

	// Dangerous drift mode must be rejected
	resDangerous, err := engine.Execute("drive.switch_mode", map[string]interface{}{
		"mode": "drift",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if resDangerous.Accepted || resDangerous.Entry.Decision != DecisionRejected {
		t.Errorf("expected rejected for drift mode, got %v", resDangerous.Entry.Decision)
	}
	if resDangerous.Entry.Reason != ReasonDangerousActionRejected {
		t.Errorf("expected %s, got %s", ReasonDangerousActionRejected, resDangerous.Entry.Reason)
	}
}

func TestMockVehicle_AuditLogFormat(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "audit.log")

	engine, err := NewEngine(logPath)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	_, _ = engine.Execute("climate.set_temperature", map[string]interface{}{"target_temperature": 22.0})
	_, _ = engine.Execute("window.control", map[string]interface{}{"action": "close", "position": "all"})

	// Read lines back and verify JSON Lines format
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		var entry AuditLogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("invalid json line: %s, err: %v", string(line), err)
		}
		if entry.Timestamp == "" {
			t.Errorf("missing timestamp")
		}
		if !entry.Simulation {
			t.Errorf("simulation must be true")
		}
		lineCount++
	}

	if lineCount != 2 {
		t.Errorf("expected 2 log lines, got %d", lineCount)
	}
}

func TestMockVehicle_NaturalSpeechExtraction(t *testing.T) {
	cases := []struct {
		input      string
		wantIntent string
		wantMatch  bool
	}{
		{"把空调调到24度", "climate.set_temperature", true},
		{"主驾空调设为26度", "climate.set_temperature", true},
		{"打开全部车窗", "window.control", true},
		{"关上天窗", "window.control", true},
		{"切换成运动模式", "drive.switch_mode", true},
		{"切换到漂移模式", "drive.switch_mode", true},
		{"今天北京天气怎么样", "", false},
		{"讲个笑话听听", "", false},
	}

	for _, c := range cases {
		intent, _, matched := ParseIntentFromText(c.input)
		if matched != c.wantMatch {
			t.Errorf("input: %s, wantMatch: %v, got: %v", c.input, c.wantMatch, matched)
		}
		if intent != c.wantIntent {
			t.Errorf("input: %s, wantIntent: %s, got: %s", c.input, c.wantIntent, intent)
		}
	}
}
