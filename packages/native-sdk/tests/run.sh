#!/usr/bin/env bash
set -euo pipefail
package_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
python3 -B -m unittest discover -s "$package_root/tests" -p 'test_*.py'
