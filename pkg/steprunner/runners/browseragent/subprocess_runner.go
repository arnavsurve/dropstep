package browseragent

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/arnavsurve/dropstep/pkg/steprunner/runners/browseragent/assets"
	"github.com/arnavsurve/dropstep/pkg/types"
)

const (
	venvDirName          = "dropstep_agent_venv"
	requirementsHashFile = ".requirements_hash"
)

// ensurePythonVenv sets up the Python virtual environment for the agent.
// It extracts requirements.txt, creates a venv if it doesn't exist or if requirements changed,
// and installs dependencies. Returns the path to the venv's python executable.
func ensurePythonVenv(baseCacheDir string, logger types.Logger) (string, string, error) {
	venvPath := filepath.Join(baseCacheDir, venvDirName)
	pythonInterpreter := filepath.Join(venvPath, "bin", "python")
	pipExecutable := filepath.Join(venvPath, "bin", "pip")

	// Get embedded requirements.txt content
	reqBytes, err := assets.GetAgentScriptContent(assets.RequirementsFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to get embedded requirements.txt: %w", err)
	}
	currentReqHash := fmt.Sprintf("%x", sha256.Sum256(reqBytes))

	// Check if venv exists and if requirements have changed
	storedReqHashPath := filepath.Join(venvPath, requirementsHashFile)
	_, venvStatErr := os.Stat(pythonInterpreter)

	recreateVenv := false
	if os.IsNotExist(venvStatErr) {
		logger.Debug().Msg("Python venv not found, creating...")
		recreateVenv = true
	} else {
		storedReqHashBytes, err := os.ReadFile(storedReqHashPath)
		if err != nil || string(storedReqHashBytes) != currentReqHash {
			logger.Debug().Msg("Requirements.txt changed or hash file missing, recreating venv...")
			recreateVenv = true
			if err := os.RemoveAll(venvPath); err != nil { // Clean up old venv before recreating
				logger.Warn().Err(err).Str("path", venvPath).Msg("Failed to remove old venv")
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
		logger.Debug().Str("command", cmdVenv.String()).Msg("Executing subprocess call")
		if err := cmdVenv.Run(); err != nil {
			return "", "", fmt.Errorf("failed to create python venv (python3 -m venv %s): %w. Stderr: %s", venvPath, err, stderrVenv.String())
		}
		logger.Info().Msg("Python venv created successfully")

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
		logger.Debug().Str("command", cmdPip.String()).Msg("Executing subprocess call")
		if err := cmdPip.Run(); err != nil {
			return "", "", fmt.Errorf("failed to install requirements (pip install -r %s): %w. Stderr: %s", tempReqFile.Name(), err, stderrPip.String())
		}
		logger.Info().Msg("Python requirements installed successfully")

		// Store the hash of the current requirements.txt
		if err := os.WriteFile(storedReqHashPath, []byte(currentReqHash), 0644); err != nil {
			logger.Warn().Err(err).Str("path", storedReqHashPath).Msg("Failed to write requirements hash")
		}
	} else {
		logger.Info().Msg("Existing Python venv found")
	}

	return venvPath, pythonInterpreter, nil
}

type SubprocessAgentRunner struct {
	agentWorkDir   string
	venvPythonPath string
}

// NewSubprocessAgentRunner initializes the runner, ensuring Python environment is set up.
func NewSubprocessAgentRunner(logger types.Logger) (*SubprocessAgentRunner, error) {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		logger.Warn().Err(err).Msg("Could not get user cache dir, using temp dir for agent")
		userCacheDir = os.TempDir()
	}
	appCacheDir := filepath.Join(userCacheDir, "dropstep")
	if err := os.MkdirAll(appCacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create app cache directory %s: %w", appCacheDir, err)
	}

	venvBasePath, venvPython, err := ensurePythonVenv(appCacheDir, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure python venv: %w", err)
	}
	_ = venvBasePath

	return &SubprocessAgentRunner{
		agentWorkDir:   appCacheDir,
		venvPythonPath: venvPython,
	}, nil
}

func (s *SubprocessAgentRunner) RunAgent(
	step types.Step, 
	rawOutputPath string, 
	schemaContent string, 
	targetDownloadDir string, 
	logger types.Logger,
	apiKey string,
) ([]byte, error) {
	// Create a temporary directory for this specific agent run to place scripts
	runTempDir, err := os.MkdirTemp(s.agentWorkDir, "agentrun-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary run directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(runTempDir); err != nil {
			logger.Warn().Str("directory", runTempDir).Err(err).Msg("Failed to remove agent run temp directory")
		}
	}()

	// Extract embedded scripts to the temporary run directory
	scriptsToExtract := []string{
		assets.RunScriptFile,
		assets.MainPyFile,
		assets.CliPyFile,
		assets.ModelsPyFile,
		assets.ActionsPyFile,
		assets.SettingsPyFile,
		assets.InitPyFile,
	}
	for _, scriptName := range scriptsToExtract {
		content, err := assets.GetAgentScriptContent(scriptName)
		if err != nil {
			return nil, fmt.Errorf("failed to get embedded script %s: %w", scriptName, err)
		}
		destPath := filepath.Join(runTempDir, scriptName)
		if err := os.WriteFile(destPath, content, 0755); err != nil {
			return nil, fmt.Errorf("failed to write embedded script %s to %s: %w", scriptName, destPath, err)
		}
	}

	extractedRunScriptPath := filepath.Join(runTempDir, assets.RunScriptFile)

	outputPath, err := filepath.Abs(rawOutputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for output file %s: %v", rawOutputPath, err)
	}
	logger.Debug().Str("path", outputPath).Msg("Resolved path for agent output")

	cmdArgs := []string{"--prompt", step.Prompt, "--out", outputPath}
	if len(step.UploadFiles) > 0 {
		cmdArgs = append(cmdArgs, "--upload-file-paths")
		for _, f := range step.UploadFiles {
			absPath, err := filepath.Abs(f.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to get abs path for upload %s: %w", f.Path, err)
			}
			cmdArgs = append(cmdArgs, absPath)
		}
	}
	if schemaContent != "" {
		cmdArgs = append(cmdArgs, "--output-schema", schemaContent)
	}
	if targetDownloadDir != "" {
		cmdArgs = append(cmdArgs, "--target-download-dir", targetDownloadDir)
	} else {
		cmdArgs = append(cmdArgs, "--target-download-dir", "./output/")
	}
	if step.AllowedDomains != nil {
		cmdArgs = append(cmdArgs, "--allowed-domains")
		cmdArgs = append(cmdArgs, step.AllowedDomains...)
	}
	if step.MaxSteps != nil {
		cmdArgs = append(cmdArgs, "--max-steps", strconv.Itoa(*step.MaxSteps))
	}
	if step.MaxFailures != nil {
		cmdArgs = append(cmdArgs, "--max-failures", strconv.Itoa(*step.MaxFailures))
	}

	cmd := exec.Command(extractedRunScriptPath, cmdArgs...)
	cmd.Env = append(os.Environ(),
		"ANONYMIZED_TELEMETRY=false",
		"OPENAI_API_KEY="+apiKey,
		"DROPSTEP_VENV_PYTHON="+s.venvPythonPath,
		"DROPSTEP_AGENT_PY_PATH="+filepath.Join(runTempDir, assets.MainPyFile),
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start agent script %s: %w", extractedRunScriptPath, err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go streamOutputStructured(stdout, &wg, "STDOUT", logger)
	go streamOutputStructured(stderr, &wg, "STDERR", logger)

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

func streamOutputStructured(r io.Reader, wg *sync.WaitGroup, source string, logger types.Logger) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		logger.Info().
			Str("source", source).
			Str("agent_line", scanner.Text()).
			Msg("Agent output")
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
			return
		}
		logger.Error().Err(err).Str("source", source).Msg("Unexpected error streaming agent output")
	}
}

