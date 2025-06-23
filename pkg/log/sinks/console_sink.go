package sinks

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/arnavsurve/dropstep/pkg/log"
	"github.com/arnavsurve/dropstep/pkg/types"
	"github.com/fatih/color"
)

type ConsoleSink struct{}

func NewConsoleSink() *ConsoleSink {
	return &ConsoleSink{}
}

func (c *ConsoleSink) Write(event *log.LogEvent) error {
	stepId := getStringField(event.Fields, "step_id")
	msg := event.Message
	source := getStringField(event.Fields, "source")
	agentLine := getStringField(event.Fields, "agent_line")
	shellLine := getStringField(event.Fields, "shell_line")
	pythonLine := getStringField(event.Fields, "python_line")
	errorMsg := getStringField(event.Fields, "error")
	levelStr := strings.ToUpper(levelToString(event.Level))
	timestampStr := event.Timestamp.Format(time.RFC3339)

	levelColorMap := map[types.Level]*color.Color{
		types.DebugLevel: color.New(color.FgCyan),
		types.InfoLevel:  color.New(color.FgGreen),
		types.WarnLevel:  color.New(color.FgYellow),
		types.ErrorLevel: color.New(color.FgRed),
		types.FatalLevel: color.New(color.FgRed, color.Bold),
	}

	levelFmt := color.New(color.FgWhite).SprintFunc()
	if lc, ok := levelColorMap[event.Level]; ok {
		levelFmt = lc.SprintFunc()
	}

	timestampFmt := color.New(color.FgWhite).SprintFunc()
	stepLabel := stepId
	if stepLabel == "" {
		stepLabel = "workflow"
	}

	var output string
	commonPrefix := fmt.Sprintf("[%s %s] %s: ",
		levelFmt(levelStr),
		timestampFmt(timestampStr),
		color.CyanString(stepLabel),
	)

	switch {
	case agentLine != "" && source != "":
		output = fmt.Sprintf("%s[agent/%s]: %s", commonPrefix, color.BlueString(source), agentLine)
	case shellLine != "" && source != "":
		output = fmt.Sprintf("%s[shell/%s]: %s", commonPrefix, color.BlueString(source), shellLine)
	case pythonLine != "" && source != "":
		output = fmt.Sprintf("%s[python/%s]: %s", commonPrefix, color.BlueString(source), pythonLine)
	case errorMsg != "":
		output = fmt.Sprintf("%s%s", commonPrefix, errorMsg)
	case msg != "":
		output = fmt.Sprintf("%s%s", commonPrefix, msg)
	default:
		fieldsStr, _ := json.MarshalIndent(event.Fields, "", "  ")
		output = fmt.Sprintf("%s%s %s", commonPrefix, msg, string(fieldsStr))
	}
	fmt.Println(output)
	return nil
}

// Helper to safely get string field from LogEvent.Fields
func getStringField(fields map[string]any, key string) string {
	if val, ok := fields[key]; ok {
		if strVal, isStr := val.(string); isStr {
			return strVal
		}
	}
	return ""
}

// Helper to convert types.Level to string
func levelToString(l types.Level) string {
	switch l {
	case types.DebugLevel:
		return "debug"
	case types.InfoLevel:
		return "info"
	case types.WarnLevel:
		return "warn"
	case types.ErrorLevel:
		return "error"
	case types.FatalLevel:
		return "fatal"
	default:
		return "unknown"
	}
}

func (c *ConsoleSink) Close() error {
	return nil // Console doesn't need closing
}
