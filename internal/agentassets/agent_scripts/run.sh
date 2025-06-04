#!/usr/bin/env bash
set -e -x # Enable debug printing for bash itself

SCRIPT_DIR_WHERE_RUN_SH_IS="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo "--- run.sh ---"
echo "BASH_SOURCE[0]=${BASH_SOURCE[0]}"
echo "dirname of BASH_SOURCE[0]=$(dirname "${BASH_SOURCE[0]}")"
echo "SCRIPT_DIR_WHERE_RUN_SH_IS=$SCRIPT_DIR_WHERE_RUN_SH_IS"
echo "DROPSTEP_VENV_PYTHON=$DROPSTEP_VENV_PYTHON"
echo "Arguments received by run.sh: $@"
echo "--- end run.sh initial prints ---"

cd "$SCRIPT_DIR_WHERE_RUN_SH_IS" # CD in the main shell of the script
echo "Changed CWD to: $(pwd)"
echo "Listing files in CWD:"
ls -la # List files to confirm main.py is here

echo "Executing: $DROPSTEP_VENV_PYTHON main.py $@"
"$DROPSTEP_VENV_PYTHON" "main.py" "$@"
echo "Python script finished."