#!/bin/sh
set -eu

BASE_URL="${FASTMD_BASE_URL:-https://fastmd.dev}"
TOKEN_PREFIX="fmd_live_"
TOKEN_PATH="${HOME}/.config/fastmd/token"
TMP_FILES=""

cleanup() {
  for file in $TMP_FILES; do
    [ -n "$file" ] && [ -f "$file" ] && rm -f "$file"
  done
}

trap cleanup EXIT INT TERM

usage() {
  cat <<'EOF'
Usage: publish.sh [--file path]

Publish Markdown to fastmd.dev and print the URL.
EOF
}

random_token() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 6
    return
  fi

  od -An -N6 -tx1 /dev/urandom 2>/dev/null | tr -d ' \n'
}

json_escape_file() {
  awk 'BEGIN { first = 1 }
  {
    gsub(/\\/, "\\\\")
    gsub(/\"/, "\\\"")
    gsub(/\t/, "\\t")
    gsub(/\r/, "\\r")
    if (!first) {
      printf "\\n"
    }
    printf "%s", $0
    first = 0
  }' "$1"
}

load_or_create_token() {
  if [ -f "$TOKEN_PATH" ]; then
    token=$(tr -d '\r\n' < "$TOKEN_PATH")
    if [ -n "$token" ]; then
      printf '%s\n' "$token"
      return
    fi
  fi

  token="${TOKEN_PREFIX}$(random_token)"
  mkdir -p "$(dirname "$TOKEN_PATH")"
  printf '%s\n' "$token" > "$TOKEN_PATH"
  printf '%s\n' "$token"
}

read_content() {
  file_path="$1"
  if [ -n "$file_path" ]; then
    cat "$file_path"
    return
  fi

  cat
}

extract_status() {
  response_file="$1"
  tail_line=$(awk 'END { print }' "$response_file")
  case "$tail_line" in
    __FASTMD_STATUS__:*)
      printf '%s\n' "${tail_line#__FASTMD_STATUS__:}"
      ;;
    *)
      printf 'invalid\n'
      ;;
  esac
}

extract_body() {
  response_file="$1"
  awk 'BEGIN { first = 1 } /^__FASTMD_STATUS__:/ { exit } { if (!first) printf "\n"; printf "%s", $0; first = 0 }' "$response_file"
}

FILE_PATH=""

while [ $# -gt 0 ]; do
  case "$1" in
    --file)
      shift
      [ $# -gt 0 ] || { echo "Error: --file requires a path" >&2; exit 1; }
      FILE_PATH="$1"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Error: unknown option '$1'" >&2
      usage >&2
      exit 1
      ;;
  esac
  shift
done

CONTENT_FILE=$(mktemp)
RESPONSE_FILE=$(mktemp)
TMP_FILES="$CONTENT_FILE $RESPONSE_FILE"

if ! read_content "$FILE_PATH" > "$CONTENT_FILE"; then
  echo "Error: failed to read input" >&2
  exit 1
fi

if [ -z "$(tr -d '[:space:]' < "$CONTENT_FILE")" ]; then
  echo "Error: content is empty" >&2
  exit 1
fi

if ! TOKEN=$(load_or_create_token); then
  echo "Error: token file failure" >&2
  exit 1
fi

PAYLOAD_FILE=$(mktemp)
TMP_FILES="$TMP_FILES $PAYLOAD_FILE"

escaped_content=$(json_escape_file "$CONTENT_FILE")
printf '{"content":"%s","token":"%s"}\n' "$escaped_content" "$TOKEN" > "$PAYLOAD_FILE"

if ! curl -sS -X POST "$BASE_URL/v1/push" -H "Content-Type: application/json" --data-binary "@$PAYLOAD_FILE" -w '\n__FASTMD_STATUS__:%{http_code}\n' > "$RESPONSE_FILE"; then
  echo "Error: network error" >&2
  exit 1
fi

STATUS=$(extract_status "$RESPONSE_FILE")
BODY=$(extract_body "$RESPONSE_FILE")

if [ "$STATUS" = "invalid" ]; then
  echo "Error: server returned an invalid HTTP status" >&2
  exit 1
fi

if [ "$STATUS" != "200" ]; then
  if [ -n "$BODY" ]; then
    echo "Error: server returned $STATUS: $BODY" >&2
  else
    echo "Error: server returned $STATUS" >&2
  fi
  exit 1
fi

URL=$(printf '%s' "$BODY" | sed -n 's/.*"url":"\([^"]*\)".*/\1/p')
if [ -z "$URL" ]; then
  echo "Error: server response did not include a URL" >&2
  exit 1
fi

printf '%s\n' "$URL"
printf 'Dashboard: %s/dashboard?token=%s\n' "$BASE_URL" "$TOKEN" >&2
