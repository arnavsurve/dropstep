#!/usr/bin/env bash
set -e

SCRIPT_DIR_WHERE_RUN_SH_IS="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

cd "$SCRIPT_DIR_WHERE_RUN_SH_IS" # CD in the main shell of the script

# echo "Executing: $DROPSTEP_VENV_PYTHON main.py $@"
"$DROPSTEP_VENV_PYTHON" "main.py" "$@"
