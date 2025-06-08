package logging

import (
	"encoding/json"
	"io"
	"os"

	"github.com/rs/zerolog"
)

type FileSink struct {
	writer io.Writer
}

func NewFileSink(path string) (*FileSink, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	return &FileSink{writer: f}, nil
}

func (f *FileSink) Write(level zerolog.Level, event map[string]any) {
	data, _ := json.Marshal(event)
	f.writer.Write(append(data, '\n'))
}