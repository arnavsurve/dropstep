package logging

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/arnavsurve/dropstep/internal/security"
	"github.com/fatih/color"
	"github.com/rs/zerolog"
)

type ConsoleSink struct{}

func (c *ConsoleSink) Write(level zerolog.Level, event map[string]any, redactor *security.Redactor) {
	for key, val := range event {
		if strVal, ok := val.(string); ok {
			event[key] = redactor.Redact(strVal)
		}
	}

	// Extract fields safely
	stepId := getString(event, "step_id")
	msg := getString(event, "message")
	source := getString(event, "source")
	agentLine := getString(event, "agent_line")
	shellLine := getString(event, "shell_line")
	pythonLine := getString(event, "python_line")
	errorMsg := getString(event, "error")
	timestamp := getString(event, "time")

	// Define colors per log level
	levelColor := map[zerolog.Level]*color.Color{
		zerolog.DebugLevel: color.New(color.FgCyan),
		zerolog.InfoLevel:  color.New(color.FgGreen),
		zerolog.WarnLevel:  color.New(color.FgYellow),
		zerolog.ErrorLevel: color.New(color.FgRed),
		zerolog.FatalLevel: color.New(color.FgRed, color.Bold),
	}

	levelFmt := levelColor[level].SprintFunc()
	timestampFmt := color.New(color.FgWhite).SprintFunc()
	stepLabel := stepId
	if stepLabel == "" {
		stepLabel = "workflow"
	}

	switch {
	case agentLine != "" && source != "":
		fmt.Printf("[%s %s] %s: [agent/%s]: %s\n",
			levelFmt(strings.ToUpper(level.String())),
			timestampFmt(timestamp),
			color.CyanString(stepLabel),
			color.BlueString(source),
			agentLine)

	case shellLine != "" && source != "":
		fmt.Printf("[%s %s] %s: [shell/%s]: %s\n",
			levelFmt(strings.ToUpper(level.String())),
			timestampFmt(timestamp),
			color.CyanString(stepLabel),
			color.BlueString(source),	
			shellLine)

	case pythonLine != "" && source != "":
		fmt.Printf("[%s %s] %s: [python/%s]: %s\n",
			levelFmt(strings.ToUpper(level.String())),
			timestampFmt(timestamp),
			color.CyanString(stepLabel),
			color.BlueString(source),	
			pythonLine)

	case errorMsg != "":
		fmt.Printf("[%s %s] %s: %s\n",
			levelFmt(strings.ToUpper(level.String())),
			timestampFmt(timestamp),
			color.RedString(stepLabel),
			errorMsg)

	case msg != "":
		fmt.Printf("[%s %s] %s: %s\n",
			levelFmt(strings.ToUpper(level.String())),
			timestampFmt(timestamp),
			color.CyanString(stepLabel),
			msg)

	default:
		// Fallback: print entire event
		jsonStr, _ := json.MarshalIndent(event, "", "  ")
		fmt.Printf("[%s %s] %s: %s\n",
			levelFmt(strings.ToUpper(level.String())),
			timestampFmt(timestamp),
			color.CyanString(stepLabel),
			string(jsonStr))
	}
}

// getString safely extracts a string from a map
func getString(m map[string]any, key string) string {
	if val, ok := m[key]; ok && val != nil {
		return fmt.Sprintf("%v", val)
	}
	return ""
}

// Close implements the io.Closer interface. We don't want to close os.Stdout,
// this is a no-op.
func (c *ConsoleSink) Close() error {
	return nil
}