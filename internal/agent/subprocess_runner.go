package agent

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/arnavsurve/dropstep/internal/agentassets"
)

const venvDirName = "dropstep_agent_venv"
const requirementsHashFile = ".requirements_hash"

// ensurePythonVenv sets up the Python virtual environment for the agent.
// It extracts requirements.txt, creates a venv if it doesn't exist or if requirements changed,
// and installs dependencies. Returns the path to the venv's python executable.
func ensurePythonVenv(baseCacheDir string) (string, string, error) {
	venvPath := filepath.Join(baseCacheDir, venvDirName)
	pythonInterpreter := filepath.Join(venvPath, "bin", "python")
	pipExecutable := filepath.Join(venvPath, "bin", "pip")

	// Get embedded requirements.txt content
	reqBytes, err := agentassets.GetAgentScriptContent(agentassets.RequirementsFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to get embedded requirements.txt: %w", err)
	}
	currentReqHash := fmt.Sprintf("%x", sha256.Sum256(reqBytes))

	// Check if venv exists and if requirements have changed
	storedReqHashPath := filepath.Join(venvPath, requirementsHashFile)
	_, venvStatErr := os.Stat(pythonInterpreter)

	recreateVenv := false
	if os.IsNotExist(venvStatErr) {
		log.Println("Python venv not found, creating...")
		recreateVenv = true
	} else {
		storedReqHashBytes, err := os.ReadFile(storedReqHashPath)
		if err != nil || string(storedReqHashBytes) != currentReqHash {
			log.Println("Requirements.txt changed or hash file missing, recreating venv...")
			recreateVenv = true
			if err := os.RemoveAll(venvPath); err != nil { // Clean up old venv before recreating
				log.Printf("Warning: failed to remove old venv at %s: %v", venvPath, err)
			}
		}
	}

	if recreateVenv {
		if err := os.MkdirAll(venvPath, 0755); err != nil {
			return "", "", fmt.Errorf("failed to create directory for venv %s: %w", venvPath, err)
		}

		// Create venv
		// Assumes 'python3' (or 'python') is in PATH and can create venvs
		cmdVenv := exec.Command("python3", "-m", "venv", venvPath)
		var stderrVenv bytes.Buffer
		cmdVenv.Stderr = &stderrVenv
		log.Printf("Executing: %s", cmdVenv.String())
		if err := cmdVenv.Run(); err != nil {
			return "", "", fmt.Errorf("failed to create python venv (python3 -m venv %s): %w. Stderr: %s", venvPath, err, stderrVenv.String())
		}
		log.Println("Python venv created successfully.")

		// Install requirements
		// Need to write requirements.txt to a temporary location for pip to read
		tempReqFile, err := os.CreateTemp(baseCacheDir, "requirements-*.txt")
		if err != nil {
			return "", "", fmt.Errorf("failed to create temporary requirements.txt: %w", err)
		}
		defer os.Remove(tempReqFile.Name()) // Clean up temp file

		if _, err := tempReqFile.Write(reqBytes); err != nil {
			tempReqFile.Close()
			return "", "", fmt.Errorf("failed to write to temporary requirements.txt: %w", err)
		}
		tempReqFile.Close() // Close before pip uses it

		cmdPip := exec.Command(pipExecutable, "install", "-r", tempReqFile.Name())
		var stderrPip bytes.Buffer
		cmdPip.Stderr = &stderrPip
		log.Printf("Executing: %s", cmdPip.String())
		if err := cmdPip.Run(); err != nil {
			return "", "", fmt.Errorf("failed to install requirements (pip install -r %s): %w. Stderr: %s", tempReqFile.Name(), err, stderrPip.String())
		}
		log.Println("Python requirements installed successfully.")

		// Store the hash of the current requirements.txt
		if err := os.WriteFile(storedReqHashPath, []byte(currentReqHash), 0644); err != nil {
			log.Printf("Warning: failed to write requirements hash to %s: %v", storedReqHashPath, err)
		}
	} else {
		log.Println("Using existing Python venv.")
	}

	return venvPath, pythonInterpreter, nil
}

type SubprocessAgentRunner struct {
	agentWorkDir string
	venvPythonPath string
}

// NewSubprocessAgentRunner initializes the runner, ensuring Python environment is set up.
func NewSubprocessAgentRunner() (*SubprocessAgentRunner, error) {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Printf("Warning: could not get user cache dir, using temp dir for agent: %v", err)
		userCacheDir = os.TempDir()
	}
	appCacheDir := filepath.Join(userCacheDir, "dropstep")
	if err := os.MkdirAll(appCacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create app cache directory %s: %w", appCacheDir, err)
	}

	venvBasePath, venvPython, err := ensurePythonVenv(appCacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure python venv: %w", err)
	}
	_ = venvBasePath

	return &SubprocessAgentRunner{
		agentWorkDir: appCacheDir,
		venvPythonPath: venvPython,
	}, nil
}

func (s *SubprocessAgentRunner) RunAgent(
	prompt,
	outputPath string,
	filesToUpload []internal.FileToUpload,
	schemaContent string,
) ([]byte, error) {
	// Create a temporary directory for this specific agent run to place scripts
	runTempDir, err := os.MkdirTemp(s.agentWorkDir, "agentrun-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary run directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(runTempDir); err != nil {
			log.Printf("Warning: failed to remove agent run temp directory %s: %v", runTempDir, err)
		}
	}()

	// Extract embedded scripts to the temporary run directory
	scriptsToExtract := []string{agentassets.RunScriptFile, agentassets.AgentPyFile}
	for _, scriptName := range scriptsToExtract {
		content, err := agentassets.GetAgentScriptContent(scriptName)
		if err != nil {
			return nil, fmt.Errorf("failed to get embedded script %s: %w", scriptName, err)
		}
		destPath := filepath.Join(runTempDir, scriptName)
		if err := os.WriteFile(destPath, content, 0755); err != nil {
			return nil, fmt.Errorf("failed to write embedded script %s to %s: %w", scriptName, destPath, err)
		}
	}

	extractedRunScriptPath := filepath.Join(runTempDir, agentassets.RunScriptFile)

	cmdArgs := []string{"--prompt", prompt, "--out", outputPath}
	if len(filesToUpload) > 0 {
		cmdArgs = append(cmdArgs, "--upload-file-paths")
		for _, f := range filesToUpload {
			absPath, err := filepath.Abs(f.Path)
			if err != nil { return nil, fmt.Errorf("failed to get abs path for upload %s: %w", f.Path, err) }
			cmdArgs = append(cmdArgs, absPath)
		}
	}
	if schemaContent != "" {
		cmdArgs = append(cmdArgs, "--output-schema", schemaContent)
	}

	cmd := exec.Command(extractedRunScriptPath, cmdArgs...)
	cmd.Env = append(os.Environ(),
		"OPENAI_API_KEY="+os.Getenv("OPENAI_API_KEY"),
		"DROPSTEP_VENV_PYTHON="+s.venvPythonPath,
		"DROPSTEP_AGENT_PY_PATH="+filepath.Join(runTempDir, agentassets.AgentPyFile),
	)

	// The command will run in the context of the OS, not within runTempDir CWD by default.
	// If run.sh relies on CWD to find agent.py, set cmd.Dir
	// cmd.Dir = runTempDir // Often a good idea

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start agent script %s: %w", extractedRunScriptPath, err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go streamOutput(stdout, os.Stdout, &wg, "AGENT_STDOUT")
	go streamOutput(stderr, os.Stderr, &wg, "AGENT_STDERR")

	waitErr := cmd.Wait()
	wg.Wait()
	if waitErr != nil {
		return nil, fmt.Errorf("agent script %s failed: %w", extractedRunScriptPath, waitErr)
	}
	jsonData, readFileErr := os.ReadFile(outputPath)
	if readFileErr != nil {
		return nil, fmt.Errorf("failed to read agent output file %s: %w", outputPath, readFileErr)
	}
	return jsonData, nil
}

func streamOutput(r io.Reader, w io.Writer, wg *sync.WaitGroup, prefix string) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		// You could add a prefix here if you want to distinguish agent stdout/stderr
		fmt.Fprintf(w, "[%s] %s\n", prefix, scanner.Text())
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "Error reading stream for %s: %v\n", prefix, err)
	}
}
