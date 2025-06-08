package logging

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
)

type ConsoleSink struct{}

func (c *ConsoleSink) Write(level zerolog.Level, event map[string]any) {
	stepId := fmt.Sprintf("%v", event["step_id"])
	msg := fmt.Sprintf("%v", event["message"])
	source := fmt.Sprintf("%v", event["source"])
	agentLine := fmt.Sprintf("%v", event["agent_line"])

	timestamp := time.Now().Format("15:04:05")

	// Define colors for different log levels
	levelColor := map[zerolog.Level]*color.Color{
		zerolog.DebugLevel: color.New(color.FgCyan),
		zerolog.InfoLevel:  color.New(color.FgGreen),
		zerolog.WarnLevel:  color.New(color.FgYellow),
		zerolog.ErrorLevel: color.New(color.FgRed),
		zerolog.FatalLevel: color.New(color.FgRed, color.Bold),
	}

	// Get color for current level
	levelFmt := levelColor[level].SprintFunc()
	timestampFmt := color.New(color.FgWhite).SprintFunc()

	switch {
	case agentLine != "<nil>" && source != "<nil>":
		// Agent subprocess output
		fmt.Printf("[%s %s] %s [agent/%s]: %s\n",
			levelFmt(strings.ToUpper(level.String())),
			timestampFmt(timestamp),
			color.CyanString(stepId),
			color.BlueString(source),
			agentLine)

	case stepId == "<nil>":
		// Base logging at workflow level, no step ID
		fmt.Printf("[%s %s] %s\n",
			levelFmt(strings.ToUpper(level.String())),
			timestampFmt(timestamp),
			msg)

	case msg != "<nil>":
		// General message within a step
		fmt.Printf("[%s %s] %s: %s\n",
			levelFmt(strings.ToUpper(level.String())),
			timestampFmt(timestamp),
			color.CyanString(stepId),
			msg)

	default:
		// Fallback for unexpected structured event
		jsonStr, _ := json.MarshalIndent(event, "", "  ")
		fmt.Printf("[%s %s] %s: %s\n",
			levelFmt(strings.ToUpper(level.String())),
			timestampFmt(timestamp),
			color.CyanString(stepId),
			string(jsonStr))
	}
}
