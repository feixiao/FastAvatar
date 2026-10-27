package mockvehicle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DecisionType represents the audit decision.
type DecisionType string

const (
	DecisionAccepted DecisionType = "MOCK_ACCEPTED"
	DecisionRejected DecisionType = "MOCK_REJECTED"
)

// Reason codes strictly per PRD specification.
const (
	ReasonValidSlotsSafeRange      = "VALID_SLOTS_AND_SAFE_RANGE"
	ReasonSlotMissing              = "ERR_SLOT_MISSING"
	ReasonTempOutOfRange           = "ERR_TEMP_OUT_OF_RANGE"
	ReasonUnsupportedWindowAction  = "ERR_UNSUPPORTED_WINDOW_ACTION"
	ReasonDangerousActionRejected  = "ERR_DANGEROUS_ACTION_REJECTED"
	ReasonUnknownIntent            = "ERR_UNKNOWN_VEHICLE_INTENT"
)

// AuditLogEntry represents one strict JSON Lines audit record.
// PRD Requirement: Must contain timestamp, intent, slots, decision, reason, simulation=true.
// Must NOT contain raw voice audio or raw transcript text.
type AuditLogEntry struct {
	Timestamp  string                 `json:"timestamp"` // UTC RFC 3339
	Intent     string                 `json:"intent"`
	Slots      map[string]interface{} `json:"slots"`
	Decision   DecisionType           `json:"decision"`
	Reason     string                 `json:"reason"`
	Simulation bool                   `json:"simulation"` // Must always be true
}

// ExecutionResult holds the in-memory decision, feedback speech, and audit entry.
type ExecutionResult struct {
	Entry        AuditLogEntry
	Confirmation string
	Accepted     bool
}

// Engine validates vehicle mock intents and appends audit logs.
type Engine struct {
	logFilePath string
	mu          sync.Mutex
}

// NewEngine creates a new mock vehicle engine with audit log persistence.
func NewEngine(customLogPath string) (*Engine, error) {
	logPath := customLogPath
	if logPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "."
		}
		avatarDir := filepath.Join(homeDir, ".avatar")
		if err := os.MkdirAll(avatarDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create avatar dir: %w", err)
		}
		logPath = filepath.Join(avatarDir, "mock_vehicle.log")
	}

	return &Engine{
		logFilePath: logPath,
	}, nil
}

// Execute evaluates a parsed vehicle intent, logs it, and returns user confirmation text.
func (e *Engine) Execute(intent string, slots map[string]interface{}) (ExecutionResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	entry := AuditLogEntry{
		Timestamp:  time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Intent:     intent,
		Slots:      slots,
		Simulation: true,
	}

	var confirmation string
	var accepted bool

	switch intent {
	case "climate.set_temperature":
		accepted, entry.Reason, confirmation = validateClimateSetTemp(slots)
	case "window.control":
		accepted, entry.Reason, confirmation = validateWindowControl(slots)
	case "drive.switch_mode":
		accepted, entry.Reason, confirmation = validateDriveSwitchMode(slots)
	default:
		accepted = false
		entry.Reason = ReasonUnknownIntent
		confirmation = "不支持的模拟车控意图。"
	}

	if accepted {
		entry.Decision = DecisionAccepted
	} else {
		entry.Decision = DecisionRejected
	}

	// Write strict single JSON Lines to audit log
	if err := e.appendAuditLog(entry); err != nil {
		return ExecutionResult{Entry: entry, Confirmation: confirmation, Accepted: accepted}, err
	}

	return ExecutionResult{
		Entry:        entry,
		Confirmation: confirmation,
		Accepted:     accepted,
	}, nil
}

func (e *Engine) appendAuditLog(entry AuditLogEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit log: %w", err)
	}

	f, err := os.OpenFile(e.logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit log file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write audit log entry: %w", err)
	}

	return nil
}

// Validation logic for climate.set_temperature (16.0 ~ 30.0 deg C)
func validateClimateSetTemp(slots map[string]interface{}) (bool, string, string) {
	val, ok := slots["target_temperature"]
	if !ok || val == nil {
		return false, ReasonSlotMissing, "未指定温度，模拟调节失败。"
	}

	var temp float64
	switch v := val.(type) {
	case float64:
		temp = v
	case int:
		temp = float64(v)
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return false, ReasonSlotMissing, "无法识别温度数值，模拟已拒绝。"
		}
		temp = parsed
	default:
		return false, ReasonSlotMissing, "温度参数格式有误，模拟已拒绝。"
	}

	if temp < 16.0 || temp > 30.0 {
		return false, ReasonTempOutOfRange, fmt.Sprintf("温度 %.1f 度超出安全范围（16~30度），模拟已拒绝。", temp)
	}

	zone := "driver"
	if z, ok := slots["zone"].(string); ok && z != "" {
		zone = z
	}

	confirm := fmt.Sprintf("已模拟将%s空调调至 %.1f 度（模拟控制，未连接真实车辆）。", formatZone(zone), temp)
	return true, ReasonValidSlotsSafeRange, confirm
}

// Validation logic for window.control (open/close/vent, positions)
func validateWindowControl(slots map[string]interface{}) (bool, string, string) {
	action, ok := slots["action"].(string)
	if !ok || action == "" {
		return false, ReasonSlotMissing, "缺少车窗操作动作，模拟已拒绝。"
	}
	action = strings.ToLower(action)

	if action != "open" && action != "close" && action != "vent" {
		return false, ReasonUnsupportedWindowAction, "不支持的车窗动作，模拟已拒绝。"
	}

	position, _ := slots["position"].(string)
	if position == "" {
		position = "all"
	}
	position = strings.ToLower(position)

	actionName := "打开"
	if action == "close" {
		actionName = "关闭"
	} else if action == "vent" {
		actionName = "微开透气"
	}

	confirm := fmt.Sprintf("已模拟%s%s（模拟控制，未连接真实车辆）。", actionName, formatWindowPosition(position))
	return true, ReasonValidSlotsSafeRange, confirm
}

// Validation logic for drive.switch_mode (eco/comfort/sport; rejects dangerous like drift/race)
func validateDriveSwitchMode(slots map[string]interface{}) (bool, string, string) {
	mode, ok := slots["mode"].(string)
	if !ok || mode == "" {
		return false, ReasonSlotMissing, "未指定驾驶模式，模拟已拒绝。"
	}
	mode = strings.ToLower(strings.TrimSpace(mode))

	// Dangerous modes strictly rejected
	if mode == "drift" || mode == "race" || mode == "track" || mode == "offroad" {
		return false, ReasonDangerousActionRejected, fmt.Sprintf("安全规则拦截：%s模式属于危险驾驶动作，模拟已拒绝。", mode)
	}

	if mode != "eco" && mode != "comfort" && mode != "sport" && mode != "standard" {
		return false, ReasonDangerousActionRejected, "不支持的驾驶模式，模拟已拒绝。"
	}

	modeName := "舒适"
	switch mode {
	case "eco":
		modeName = "节能"
	case "sport":
		modeName = "运动"
	case "standard", "comfort":
		modeName = "舒适"
	}

	confirm := fmt.Sprintf("已模拟切换至%s驾驶模式（模拟控制，未连接真实车辆）。", modeName)
	return true, ReasonValidSlotsSafeRange, confirm
}

func formatZone(zone string) string {
	switch zone {
	case "driver":
		return "主驾"
	case "passenger":
		return "副驾"
	case "all":
		return "全车"
	default:
		return zone
	}
}

func formatWindowPosition(pos string) string {
	switch pos {
	case "driver":
		return "主驾驶车窗"
	case "passenger":
		return "副驾驶车窗"
	case "sunshade":
		return "遮阳帘"
	case "sunroof":
		return "天窗"
	case "all":
		return "全部车窗"
	default:
		return pos + "车窗"
	}
}

// Quick Slot Extractor Helpers
var (
	reTemp    = regexp.MustCompile(`(?:把?空调?|温度)?(?:调[到至为]|设[为至]|设置[为至到])?\s*(\d+(?:\.\d+)?)\s*(?:度|℃)?`)
	reWindow  = regexp.MustCompile(`(打?开|关[闭上]?|透气|降下|升起)\s*(主驾|副驾|天窗|遮阳帘|全部?|所有)?\s*(车窗|天窗|窗户|遮阳帘)?`)
	reDrive   = regexp.MustCompile(`(?:切换|调到?|设置?为?)\s*(节能|省油|运动|舒适|标准|漂移|赛道)\s*模式?`)
)

// ParseIntentFromText extracts vehicle intent & slots from natural speech in < 1ms.
func ParseIntentFromText(text string) (intent string, slots map[string]interface{}, matched bool) {
	clean := strings.TrimSpace(text)

	// 1. Check climate
	if strings.Contains(clean, "空调") || strings.Contains(clean, "温度") || strings.Contains(clean, "冷气") || strings.Contains(clean, "暖气") {
		slots = make(map[string]interface{})
		if strings.Contains(clean, "主驾") {
			slots["zone"] = "driver"
		} else if strings.Contains(clean, "副驾") {
			slots["zone"] = "passenger"
		} else if strings.Contains(clean, "全车") {
			slots["zone"] = "all"
		}

		if m := reTemp.FindStringSubmatch(clean); len(m) > 1 && m[1] != "" {
			if t, err := strconv.ParseFloat(m[1], 64); err == nil {
				slots["target_temperature"] = t
			}
		}
		return "climate.set_temperature", slots, true
	}

	// 2. Check window
	if strings.Contains(clean, "车窗") || strings.Contains(clean, "天窗") || strings.Contains(clean, "窗户") || strings.Contains(clean, "遮阳帘") {
		slots = make(map[string]interface{})
		action := "open"
		if strings.Contains(clean, "关") || strings.Contains(clean, "升") {
			action = "close"
		} else if strings.Contains(clean, "透气") {
			action = "vent"
		}
		slots["action"] = action

		position := "all"
		if strings.Contains(clean, "主驾") {
			position = "driver"
		} else if strings.Contains(clean, "副驾") {
			position = "passenger"
		} else if strings.Contains(clean, "天窗") {
			position = "sunroof"
		} else if strings.Contains(clean, "遮阳帘") {
			position = "sunshade"
		}
		slots["position"] = position

		return "window.control", slots, true
	}

	// 3. Check drive mode
	if strings.Contains(clean, "模式") && (strings.Contains(clean, "驾驶") || strings.Contains(clean, "运动") || strings.Contains(clean, "节能") || strings.Contains(clean, "舒适") || strings.Contains(clean, "漂移") || strings.Contains(clean, "赛道")) {
		slots = make(map[string]interface{})
		mode := "comfort"
		if strings.Contains(clean, "运动") {
			mode = "sport"
		} else if strings.Contains(clean, "节能") || strings.Contains(clean, "省油") {
			mode = "eco"
		} else if strings.Contains(clean, "漂移") {
			mode = "drift"
		} else if strings.Contains(clean, "赛道") {
			mode = "race"
		}
		slots["mode"] = mode
		return "drive.switch_mode", slots, true
	}

	return "", nil, false
}
