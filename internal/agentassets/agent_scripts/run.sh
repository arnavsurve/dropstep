#!/usr/bin/env bash
set -e # Exit immediately if a command exits with a non-zero status.

# DROPSTEP_VENV_PYTHON should be like /path/to/cache/dropstep_agent_venv/bin/python
# DROPSTEP_AGENT_PY_PATH should be like /path/to/temp_run_dir/agent.py

if [ -z "$DROPSTEP_VENV_PYTHON" ]; then
  echo "Error: DROPSTEP_VENV_PYTHON environment variable is not set." >&2
  exit 1
fi

if [ -z "$DROPSTEP_AGENT_PY_PATH" ]; then
  echo "Error: DROPSTEP_AGENT_PY_PATH environment variable is not set." >&2
  exit 1
fi

echo "Using Python interpreter: $DROPSTEP_VENV_PYTHON"
echo "Running agent script: $DROPSTEP_AGENT_PY_PATH"

# Execute agent.py using the specific python from the venv
"$DROPSTEP_VENV_PYTHON" "$DROPSTEP_AGENT_PY_PATH" "$@"