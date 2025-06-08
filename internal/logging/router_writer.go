package logging

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"
)

type RouterWriter struct {
	Router *LoggerRouter
}

func (rw *RouterWriter) Write(p []byte) (n int, err error) {
	p = bytes.TrimSpace(p)
	if len(p) == 0 {
		return len(p), nil
	}

	var event map[string]any
	if err := json.Unmarshal(p, &event); err != nil {
		fmt.Printf("Failed to parse log line: %s\n", string(p))
		return len(p), nil
	}

	levelStr, ok := event["level"].(string)
	level := zerolog.InfoLevel
	if ok {
		if parsedLevel, err := zerolog.ParseLevel(levelStr); err == nil {
			level = parsedLevel
		}
	}

	rw.Router.Log(level, event)
	return len(p), nil
}