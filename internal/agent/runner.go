package agent

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// RunAgent executes the agent script, streams its stdout/stderr,
// and captures structured JSON output to a temporary file.
// It returns the content of the JSON output file if successful, or an error.
func RunAgent(prompt string, outputPath string) ([]byte, error) {
	cmd := exec.Command("internal/agent/run.sh", "--task", prompt, "--out", outputPath)
	cmd.Env = append(os.Environ(),
		"OPENAI_API_KEY="+os.Getenv("OPENAI_API_KEY"),
	)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stderr pipe: %w", err)
	}

	fmt.Println("Launching Browser Use agent...")
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting agent command: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine to stream stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			fmt.Println(scanner.Text()) // Stream to application's stdout
		}
		if err := scanner.Err(); err != nil {
			if err != io.EOF {
				fmt.Fprintln(os.Stderr, "error reading agent stdout:", err)
			}
		}
	}()

	// Goroutine to stream stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			fmt.Fprintln(os.Stderr, scanner.Text()) // Stream to application's stderr
		}
		if err := scanner.Err(); err != nil {
			if err != io.EOF {
				fmt.Fprintln(os.Stderr, "error reading agent stderr:", err)
			}
		}
	}()

	cmdErr := cmd.Wait()
	wg.Wait()

	if cmdErr != nil {
		return nil, fmt.Errorf("agent command failed: %w", cmdErr)
	}

	jsonData, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON output file %s: %w", outputPath, err)
	}
	return jsonData, nil
}
