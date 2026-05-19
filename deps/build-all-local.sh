#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
DEPS_DIR="$SCRIPT_DIR"

echo "=== V8 Local Build (all platforms) ==="
echo "Root: $ROOT_DIR"
echo ""

# --- Prerequisites ---

check_prereqs() {
  local missing=0

  if ! command -v python3 &>/dev/null; then
    echo "ERROR: python3 not found"
    missing=1
  fi

  if ! command -v git &>/dev/null; then
    echo "ERROR: git not found"
    missing=1
  fi

  if ! command -v docker &>/dev/null; then
    echo "ERROR: docker not found (needed for linux builds)"
    missing=1
  fi

  if [[ "$(uname)" == "Darwin" ]] && ! xcode-select -p &>/dev/null; then
    echo "ERROR: Xcode Command Line Tools not installed"
    echo "  Run: xcode-select --install"
    missing=1
  fi

  if [[ $missing -ne 0 ]]; then
    echo ""
    echo "Fix the above issues and re-run."
    exit 1
  fi

  echo "[OK] Prerequisites satisfied"
}

# --- Fetch depot_tools ---

fetch_depot_tools() {
  if [[ -d "$DEPS_DIR/depot_tools/.git" ]]; then
    echo "[OK] depot_tools already present"
  else
    echo "[..] Cloning depot_tools..."
    git clone --depth=1 https://chromium.googlesource.com/chromium/tools/depot_tools.git "$DEPS_DIR/depot_tools"
    echo "[OK] depot_tools cloned"
  fi
  export PATH="$DEPS_DIR/depot_tools:$PATH"
}

# --- Fetch V8 source ---

fetch_v8() {
  local v8_hash
  v8_hash=$(cat "$DEPS_DIR/v8_hash")
  echo "V8 commit: $v8_hash"

  if [[ -d "$DEPS_DIR/v8/.git" ]]; then
    local current_hash
    current_hash=$(git -C "$DEPS_DIR/v8" rev-parse HEAD 2>/dev/null || echo "")
    if [[ "$current_hash" == "$v8_hash" ]]; then
      echo "[OK] V8 source already at correct commit"
      return
    fi
    echo "[..] V8 source at wrong commit, re-fetching..."
    rm -rf "$DEPS_DIR/v8"
  fi

  echo "[..] Fetching V8 source (this takes a while the first time)..."
  mkdir -p "$DEPS_DIR/v8"
  cd "$DEPS_DIR/v8"
  git init
  git remote add origin https://chromium.googlesource.com/v8/v8.git
  git fetch --depth=1 origin "$v8_hash"
  git checkout FETCH_HEAD
  cd "$DEPS_DIR"

  echo "[..] Running gclient sync..."
  gclient sync --delete_unversioned_trees --no-history --spec "
solutions = [
  {
    'name': 'v8',
    'url': 'https://chromium.googlesource.com/v8/v8.git',
    'deps_file': 'DEPS',
    'managed': False,
    'custom_deps': {
      'v8/testing/gmock': None,
      'v8/test/wasm-js': None,
      'v8/third_party/colorama/src': None,
      'v8/tools/gyp': None,
      'v8/tools/luci-go': None,
      'v8/third_party/catapult': None,
      'v8/third_party/android_tools': None,
    },
    'custom_vars': {
      'build_for_node': True,
    },
  },
]
"
  echo "[OK] V8 source ready"
}

# --- Build a single platform ---

build_platform() {
  local target_os="$1"
  local target_arch="$2"
  echo ""
  echo "=== Building $target_os/$target_arch ==="

  if [[ "$target_os" == "darwin" ]]; then
    cd "$DEPS_DIR"
    python3 build.py --os darwin --arch "$target_arch" -v
  elif [[ "$target_os" == "linux" ]]; then
    local platform_flag=""
    if [[ "$target_arch" == "amd64" ]]; then
      platform_flag="--platform linux/amd64"
    else
      platform_flag="--platform linux/arm64"
    fi

    docker build -t v8go-builder -f "$DEPS_DIR/Dockerfile.builder" "$DEPS_DIR"

    docker run --rm $platform_flag \
      -v "$ROOT_DIR:/work" \
      -w /work/deps \
      -e PATH="/work/deps/depot_tools:\$PATH" \
      v8go-builder \
      python3 build.py --os linux --arch "$target_arch" -v
  fi

  echo "[OK] $target_os/$target_arch complete"
}

# --- Regenerate CGo files ---

regenerate_cgo() {
  echo ""
  echo "=== Regenerating CGo files ==="
  cd "$DEPS_DIR"
  python3 update_cgo.py \
    --root-module="github.com/ChessCom/v8go" \
    --manifest-paths="*_*/libmanifest"
  echo "[OK] CGo files regenerated"
}

# --- Main ---

main() {
  local targets="${1:-all}"

  check_prereqs
  fetch_depot_tools
  fetch_v8

  case "$targets" in
    all)
      echo ""
      echo "Building all 4 platforms..."
      echo "  darwin builds run natively, linux builds run in Docker."
      echo ""

      # Darwin builds (can run in parallel)
      build_platform darwin arm64 &
      local pid_darwin_arm64=$!

      build_platform darwin amd64 &
      local pid_darwin_amd64=$!

      wait $pid_darwin_arm64
      wait $pid_darwin_amd64

      # Linux builds (can run in parallel)
      build_platform linux arm64 &
      local pid_linux_arm64=$!

      build_platform linux amd64 &
      local pid_linux_amd64=$!

      wait $pid_linux_arm64
      wait $pid_linux_amd64
      ;;
    darwin-arm64)  build_platform darwin arm64 ;;
    darwin-amd64)  build_platform darwin amd64 ;;
    linux-arm64)   build_platform linux arm64 ;;
    linux-amd64)   build_platform linux amd64 ;;
    *)
      echo "Usage: $0 [all|darwin-arm64|darwin-amd64|linux-arm64|linux-amd64]"
      exit 1
      ;;
  esac

  regenerate_cgo

  echo ""
  echo "=== Done ==="
  echo "Built platforms are in deps/{os}_{arch}/"
  echo ""
  echo "Next steps:"
  echo "  1. Remove -DV8_ENABLE_SANDBOX from cgo.go"
  echo "  2. Update go.mod (replace tommie/v8go/deps/* with local ./deps/*)"
  echo "  3. Run: go mod tidy && go test ./..."
}

main "${1:-all}"
