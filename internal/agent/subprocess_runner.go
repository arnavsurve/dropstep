package agent

import (
	"bufio"
	"fmt"
	"github.com/arnavsurve/dropstep/internal"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

type SubprocessAgentRunner struct {
	ScriptPath string
}

func (s *SubprocessAgentRunner) RunAgent(prompt, outputPath string, filesToUpload []internal.FileToUpload) ([]byte, error) {
	cmdArgs := []string{"--prompt", prompt, "--out", outputPath}

	if len(filesToUpload) > 0 {
		cmdArgs = append(cmdArgs, "--upload-file-paths")

		for _, f := range filesToUpload {
			absPath, err := filepath.Abs(f.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to get absolute path for %s: %w", f.Path, err)
			}
			cmdArgs = append(cmdArgs, absPath)
		}
	}

	cmd := exec.Command(s.ScriptPath, cmdArgs...)
	cmd.Env = append(os.Environ(),
		"OPENAI_API_KEY="+os.Getenv("OPENAI_API_KEY"),
	)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go stream(stdout, os.Stdout, &wg)
	go stream(stderr, os.Stderr, &wg)

	err := cmd.Wait()
	wg.Wait()
	if err != nil {
		return nil, fmt.Errorf("agent error: %w", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}
	return data, nil
}

func stream(r io.Reader, w io.Writer, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fmt.Fprintln(w, scanner.Text())
	}
}
