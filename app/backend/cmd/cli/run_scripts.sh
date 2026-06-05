#!/bin/bash
set -euo pipefail  # Strict error handling

# Text formatting
BOLD=$(tput bold)
NORMAL=$(tput sgr0)
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color
CHECKMARK="\xE2\x9C\x94"

COMMAND=${1:-}  # Get first argument, or empty if not provided

cd ./backend

# Check the OS
OS=$(uname)

# Function to make the script executable, works for Unix-like systems only
make_executable() {
    if [[ "$OS" == "Linux" || "$OS" == "Darwin" ]]; then
        chmod +x "$1"  # Make the script executable on Linux/macOS
    elif [[ "$OS" == "CYGWIN"* || "$OS" == "MINGW"* || "$OS" == "MSYS"* ]]; then
        echo "Skipping chmod on Windows. Ensure scripts have execute permissions."
        echo "  backend/cmd/cli/run_scripts.sh"
        echo "  backend/cmd/cli/generate_queries.sh"
    else
        echo "Unknown OS: $OS. Skipping chmod."
    fi
}

# Function to make a script executable and run it with required second and optional third arguments
run_script() {
    # Check if the script exists before attempting to make it executable
    if [[ -f "$1" ]]; then
        make_executable "$1"
        if [[ -n "${3+x}" ]]; then
            "$1" "$2" "$3" 
        elif [[ -n "${2+x}" ]]; then
            "$1" "$2"
        else
            "$1" 
        fi
    else
        echo -e "${RED}${BOLD}[Error]${NORMAL}${NC} Script '$1' not found"
        exit 1
    fi
}

echo -e "${BOLD}[Command]${NORMAL} $1"

case "$1" in
    "generate")
        run_script ./cmd/cli/generate_queries.sh
        ;;
    *)
        echo "Invalid command"
        exit 1
        ;;
esac

exit 0