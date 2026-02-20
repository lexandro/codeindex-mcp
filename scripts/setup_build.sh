#!/usr/bin/env bash
# Build environment setup for codeindex-mcp on Linux/macOS.
#
# Two build variants:
#   make build       Lightweight — Go AST only, no CGo, no GCC required (~18 MB)
#   make build-ast   Full — all 4 languages via tree-sitter, CGo + GCC required (~31 MB)
#
# This script checks Go and make (always required), and GCC (only for build-ast).
#
# Usage:
#   bash scripts/setup_build.sh

set -euo pipefail

# ── Required versions ──────────────────────────────────────────────────────────
MIN_GO_MAJOR=1
MIN_GO_MINOR=25

# ── Colors ─────────────────────────────────────────────────────────────────────
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'
CYAN='\033[0;36m'; GRAY='\033[0;90m'; BOLD='\033[1m'; NC='\033[0m'

ok()   { echo -e "  ${GREEN}[OK]${NC} $*"; }
warn() { echo -e "  ${YELLOW}[!!]${NC} $*"; }
err()  { echo -e "  ${RED}[X] ${NC} $*"; }
info() { echo -e "       ${GRAY}$*${NC}"; }
hdr()  { echo ""; echo -e "${CYAN}── $*${NC}"; }

ask_install() {
    local what="$1"
    read -r -p "  --> Install ${what} automatically? [Y/n] " ans
    [[ "$ans" == "" || "$ans" =~ ^[Yy] ]]
}

# ── Package manager detection ──────────────────────────────────────────────────
detect_pkg_manager() {
    if   command -v apt-get &>/dev/null; then echo "apt"
    elif command -v dnf     &>/dev/null; then echo "dnf"
    elif command -v pacman  &>/dev/null; then echo "pacman"
    elif command -v zypper  &>/dev/null; then echo "zypper"
    elif command -v brew    &>/dev/null; then echo "brew"
    else                                      echo "unknown"
    fi
}

PKG_MGR=$(detect_pkg_manager)

pkg_install() {
    # Usage: pkg_install <apt-pkg> [dnf-pkg] [pacman-pkg] [brew-pkg]
    local apt_pkg="${1:-}"
    local dnf_pkg="${2:-$apt_pkg}"
    local pac_pkg="${3:-$apt_pkg}"
    local brew_pkg="${4:-$apt_pkg}"

    case "$PKG_MGR" in
        apt)    sudo apt-get install -y "$apt_pkg" ;;
        dnf)    sudo dnf install -y "$dnf_pkg" ;;
        pacman) sudo pacman -S --noconfirm "$pac_pkg" ;;
        zypper) sudo zypper install -y "$apt_pkg" ;;
        brew)   brew install "$brew_pkg" ;;
        *)
            err "Unknown package manager. Please install '$apt_pkg' manually."
            return 1
            ;;
    esac
}

# ── Go ─────────────────────────────────────────────────────────────────────────
go_version_ok() {
    command -v go &>/dev/null || return 1
    local raw major minor
    raw=$(go version 2>&1)
    if [[ $raw =~ go([0-9]+)\.([0-9]+) ]]; then
        major="${BASH_REMATCH[1]}"
        minor="${BASH_REMATCH[2]}"
        [ "$major" -gt "$MIN_GO_MAJOR" ] || \
        ( [ "$major" -eq "$MIN_GO_MAJOR" ] && [ "$minor" -ge "$MIN_GO_MINOR" ] )
    else
        return 1
    fi
}

install_go_snap() {
    if command -v snap &>/dev/null; then
        sudo snap install go --classic
        # Snap bin is usually in PATH already; add if not
        if [[ ":$PATH:" != *":/snap/bin:"* ]]; then
            export PATH="/snap/bin:$PATH"
            echo 'export PATH="/snap/bin:$PATH"' >> "$HOME/.profile"
            info "Added /snap/bin to PATH in ~/.profile"
        fi
        return 0
    fi
    return 1
}

install_go_official() {
    # Download and install the official Go tarball for linux/amd64 or arm64
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64)  arch="amd64" ;;
        aarch64) arch="arm64" ;;
        *)       err "Unsupported architecture: $arch"; return 1 ;;
    esac

    local os="linux"
    if [[ "$(uname -s)" == "Darwin" ]]; then os="darwin"; fi

    # Fetch the latest version number
    local latest
    latest=$(curl -fsSL "https://go.dev/VERSION?m=text" 2>/dev/null | head -1 || echo "")
    if [[ -z "$latest" ]]; then
        err "Could not fetch latest Go version from go.dev. Check your internet connection."
        return 1
    fi

    local tarball="${latest}.${os}-${arch}.tar.gz"
    local url="https://go.dev/dl/${tarball}"

    info "Downloading ${tarball} ..."
    curl -fsSL "$url" -o "/tmp/${tarball}"
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "/tmp/${tarball}"
    rm "/tmp/${tarball}"

    # Add to PATH
    if [[ ":$PATH:" != *":/usr/local/go/bin:"* ]]; then
        export PATH="/usr/local/go/bin:$PATH"
        local profile="$HOME/.profile"
        if [[ -f "$HOME/.bashrc" ]]; then profile="$HOME/.bashrc"; fi
        if [[ -f "$HOME/.zshrc"  ]]; then profile="$HOME/.zshrc";  fi
        echo 'export PATH="/usr/local/go/bin:$PATH"' >> "$profile"
        info "Added /usr/local/go/bin to PATH in $profile"
    fi
}

check_go() {
    hdr "Go"

    if go_version_ok; then
        ok "$(go version)"
        return 0
    fi

    if command -v go &>/dev/null; then
        err "Go $(go version 2>&1) found but ${MIN_GO_MAJOR}.${MIN_GO_MINOR}+ required."
    else
        err "Go not found."
    fi

    if ! ask_install "Go ${MIN_GO_MAJOR}.${MIN_GO_MINOR}+"; then
        warn "Manual install: https://go.dev/dl/"
        return 1
    fi

    # Try snap first (Ubuntu), then official tarball
    local installed=false
    if [[ "$PKG_MGR" == "brew" ]]; then
        brew install go && installed=true
    elif install_go_snap 2>/dev/null; then
        installed=true
    else
        install_go_official && installed=true
    fi

    if $installed && go_version_ok; then
        ok "$(go version)"
        return 0
    fi

    err "Go installation failed. Please install manually: https://go.dev/dl/"
    return 1
}

# ── GCC ────────────────────────────────────────────────────────────────────────
check_gcc() {
    hdr "GCC (C compiler for CGo)"

    if command -v gcc &>/dev/null; then
        ok "$(gcc --version | head -1)"
        return 0
    fi

    err "GCC not found."

    local install_pkg=""
    case "$PKG_MGR" in
        apt)    install_pkg="build-essential" ;;
        dnf)    install_pkg="gcc make" ;;
        pacman) install_pkg="base-devel" ;;
        zypper) install_pkg="gcc make" ;;
        brew)   install_pkg="gcc" ;;
        *)      install_pkg="gcc" ;;
    esac

    if ! ask_install "$install_pkg"; then
        warn "Manual install:"
        case "$PKG_MGR" in
            apt)    info "  sudo apt-get install build-essential" ;;
            dnf)    info "  sudo dnf install gcc make" ;;
            pacman) info "  sudo pacman -S base-devel" ;;
            brew)   info "  brew install gcc" ;;
            *)      info "  Install gcc via your package manager" ;;
        esac
        return 1
    fi

    case "$PKG_MGR" in
        apt)    sudo apt-get install -y build-essential ;;
        dnf)    sudo dnf install -y gcc make ;;
        pacman) sudo pacman -S --noconfirm base-devel ;;
        zypper) sudo zypper install -y gcc make ;;
        brew)   brew install gcc ;;
        *)      err "Cannot auto-install on unknown package manager."; return 1 ;;
    esac

    if command -v gcc &>/dev/null; then
        ok "$(gcc --version | head -1)"
        return 0
    fi

    err "GCC still not found after install."
    return 1
}

# ── make ───────────────────────────────────────────────────────────────────────
check_make() {
    hdr "make"

    if command -v make &>/dev/null; then
        ok "$(make --version | head -1)"
        return 0
    fi

    warn "make not found."

    if ! ask_install "make"; then
        info "Install via package manager, then re-run."
        return 1  # non-fatal
    fi

    case "$PKG_MGR" in
        apt)    sudo apt-get install -y make ;;
        dnf)    sudo dnf install -y make ;;
        pacman) sudo pacman -S --noconfirm make ;;
        zypper) sudo zypper install -y make ;;
        brew)   brew install make ;;
        *)      err "Cannot auto-install make."; return 1 ;;
    esac

    if command -v make &>/dev/null; then
        ok "$(make --version | head -1)"
        return 0
    fi

    warn "make still not found. You can still build without it (see below)."
    return 1
}

# ── Main ───────────────────────────────────────────────────────────────────────
echo ""
echo -e "${CYAN}${BOLD}╔════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}${BOLD}║  codeindex-mcp  —  build setup (Linux)    ║${NC}"
echo -e "${CYAN}${BOLD}╚════════════════════════════════════════════╝${NC}"
echo ""
info "Detected package manager: ${PKG_MGR}"

go_ok=true;   check_go   || go_ok=false
gcc_ok=true;  check_gcc  || gcc_ok=false
make_ok=true; check_make || make_ok=false

# ── Summary ────────────────────────────────────────────────────────────────────
hdr "Summary"

if ! $go_ok; then
    err "Go ${MIN_GO_MAJOR}.${MIN_GO_MINOR}+ is required."
    echo ""
    warn "Fix the error above, then re-run this script."
    echo ""
    exit 1
fi

echo ""

if $make_ok; then
    echo -e "  ${YELLOW}Lightweight build${GRAY}  (no GCC needed, Go AST only):${NC}"
    echo -e "    ${CYAN}make build${NC}"
    echo -e "    ${CYAN}make test${NC}"
    echo ""
    if $gcc_ok; then
        echo -e "  ${YELLOW}Full AST build${GRAY}  (GCC ready, all 4 languages):${NC}"
        echo -e "    ${CYAN}make build-ast${NC}"
        echo -e "    ${CYAN}make test-ast${NC}"
    else
        echo -e "  ${YELLOW}Full AST build${GRAY}  (TypeScript/Python/JavaScript, requires GCC):${NC}"
        warn "GCC not installed — re-run this script to install it, then use 'make build-ast'."
    fi
else
    echo -e "  ${YELLOW}Lightweight:${NC}  go build -o codeindex-mcp ."
    if $gcc_ok; then
        echo -e "  ${YELLOW}Full AST:${NC}     CGO_ENABLED=1 go build -tags ast -o codeindex-mcp ."
    fi
fi

echo ""
