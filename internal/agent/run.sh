#!/usr/bin/env bash
source "$(dirname "$0")/venv/bin/activate"
python "$(dirname "$0")/agent.py" "$@"
