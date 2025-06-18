package logging

import (
	"encoding/json"
	"os"

	"github.com/arnavsurve/dropstep/internal/security"
	"github.com/rs/zerolog"
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

func (f *FileSink) Write(level zerolog.Level, event map[string]any, redactor *security.Redactor) {
	for key, val := range event {
		if strVal, ok := val.(string); ok {
			event[key] = redactor.Redact(strVal)
		}
	}
	data, _ := json.Marshal(event)
	f.file.Write(append(data, '\n'))
}

func (f *FileSink) Close() error {
	return f.file.Close()
}
