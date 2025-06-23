package sinks

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/arnavsurve/dropstep/pkg/log"
)

type FileSink struct {
	file *os.File
}

func NewFileSink(path string) (*FileSink, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	return &FileSink{file: f}, nil
}

func (fs *FileSink) Write(event *log.LogEvent) error {
	logEntry := map[string]any{
		"level":   levelToString(event.Level),
		"time":    event.Timestamp,
		"message": event.Message,
	}
	for k, v := range event.Fields {
		logEntry[k] = v
	}

	data, err := json.Marshal(logEntry)
	if err != nil {
		return fmt.Errorf("failed to marshal log event for file sink: %w", err)
	}

	if _, err := fs.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write to file sink: %w", err)
	}

	return nil
}

func (fs *FileSink) Close() error {
	if fs.file != nil {
		return fs.file.Close()
	}
	return nil
}
