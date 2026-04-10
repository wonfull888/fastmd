#!/bin/sh
set -e

REPO="wonfull888/fastmd"
REF="${FASTMD_SKILL_REF:-main}"
SKILL_NAME="fastmd"
BASE_URL="https://raw.githubusercontent.com/${REPO}/${REF}/skills/${SKILL_NAME}"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
LOCAL_SKILL_DIR="$SCRIPT_DIR/skills/$SKILL_NAME"

INSTALL_CLAUDE=0
INSTALL_OPENCODE=0
INSTALL_CODEX=0

usage() {
  cat <<'EOF'
Usage: install-skill.sh [--claude] [--opencode] [--codex] [--all]

Installs the fastmd skill for one or more supported clients.

Options:
  --claude     Install to ~/.claude/skills/fastmd
  --opencode   Install to ~/.config/opencode/skills/fastmd
  --codex      Install to ~/.codex/skills/fastmd
  --all        Install to all supported clients (default)
  -h, --help   Show this help
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --claude)
      INSTALL_CLAUDE=1
      ;;
    --opencode)
      INSTALL_OPENCODE=1
      ;;
    --codex)
      INSTALL_CODEX=1
      ;;
    --all)
      INSTALL_CLAUDE=1
      INSTALL_OPENCODE=1
      INSTALL_CODEX=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
  shift
done

if [ "$INSTALL_CLAUDE" -eq 0 ] && [ "$INSTALL_OPENCODE" -eq 0 ] && [ "$INSTALL_CODEX" -eq 0 ]; then
  INSTALL_CLAUDE=1
  INSTALL_OPENCODE=1
  INSTALL_CODEX=1
fi

download_file() {
  url="$1"
  target="$2"
  mkdir -p "$(dirname "$target")"
  curl -fsSL "$url" -o "$target"
}

install_target() {
  root="$1"
  mkdir -p "$root/scripts"
  if [ -f "$LOCAL_SKILL_DIR/SKILL.md" ] && [ -f "$LOCAL_SKILL_DIR/scripts/publish.sh" ]; then
    cp "$LOCAL_SKILL_DIR/SKILL.md" "$root/SKILL.md"
    cp "$LOCAL_SKILL_DIR/scripts/publish.sh" "$root/scripts/publish.sh"
  else
    download_file "${BASE_URL}/SKILL.md" "$root/SKILL.md"
    download_file "${BASE_URL}/scripts/publish.sh" "$root/scripts/publish.sh"
  fi
  chmod +x "$root/scripts/publish.sh"
  echo "Installed ${SKILL_NAME} to $root"
}

echo "Installing ${SKILL_NAME} skill from ${REPO}@${REF}"

if [ "$INSTALL_CLAUDE" -eq 1 ]; then
  install_target "$HOME/.claude/skills/${SKILL_NAME}"
fi

if [ "$INSTALL_OPENCODE" -eq 1 ]; then
  install_target "$HOME/.config/opencode/skills/${SKILL_NAME}"
fi

if [ "$INSTALL_CODEX" -eq 1 ]; then
  install_target "$HOME/.codex/skills/${SKILL_NAME}"
fi

echo "Done. You can now invoke the skill as '${SKILL_NAME}'."
