#!/bin/sh
set -eu
umask 077

BASE_URL="${FASTMD_BASE_URL:-https://fastmd.dev}"
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
Usage: publish.sh [--file path] [--update ID]

Publish Markdown, or replace an existing document using its original token.
Prints the document URL; --update preserves the existing URL.
EOF
}

json_escape_file() {
  # Escape bytes so backslashes, control characters and trailing newlines survive.
  od -An -v -tu1 "$1" | LC_ALL=C awk '{
    for (i = 1; i <= NF; i++) {
      n = $i + 0
      if (n == 34) printf "\\\""
      else if (n == 92) printf "\\\\"
      else if (n < 32) printf "\\u%04x", n
      else printf "%c", n
    }
  }'
}

load_or_create_token() {
  if [ -n "${FASTMD_API_TOKEN:-}" ]; then
    printf '%s\n' "$FASTMD_API_TOKEN"
    return
  fi
  if command -v python3 >/dev/null 2>&1 && [ -f "${HOME}/.config/fastmd/credentials.json" ]; then
    token=$(python3 - "$BASE_URL" <<'PYTOKEN'
import json, os, sys
with open(os.path.join(os.path.expanduser('~'), '.config/fastmd/credentials.json')) as f:
    value = json.load(f).get('servers', {}).get(sys.argv[1].rstrip('/'), {}).get('token', '')
if value:
    print(value)
PYTOKEN
    )
    if [ -n "$token" ]; then printf '%s\n' "$token"; return; fi
  fi
  echo "Error: run fastmd login first, or set FASTMD_API_TOKEN to your account's token; no token was generated" >&2
  return 1
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
UPDATE_ID=""

while [ $# -gt 0 ]; do
  case "$1" in
    --file)
      shift
      [ $# -gt 0 ] || { echo "Error: --file requires a path" >&2; exit 1; }
      FILE_PATH="$1"
      ;;
    --update)
      shift
      [ $# -gt 0 ] || { echo "Error: --update requires a document ID" >&2; exit 1; }
      UPDATE_ID="$1"
      case "$UPDATE_ID" in
        ''|*[!a-zA-Z0-9]*)
          echo "Error: use the document ID, without URL or .md suffix" >&2
          exit 1
          ;;
      esac
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
printf '{"content":"%s"}\n' "$escaped_content" > "$PAYLOAD_FILE"

# Keep the token out of the curl argument list.
AUTH_FILE=$(mktemp)
TMP_FILES="$TMP_FILES $AUTH_FILE"
printf 'Authorization: Bearer %s\n' "$TOKEN" > "$AUTH_FILE"
if [ -n "$UPDATE_ID" ]; then
  set -- -X PUT "$BASE_URL/v1/$UPDATE_ID" -H "@$AUTH_FILE"
else
  set -- -X POST "$BASE_URL/v1/push" -H "@$AUTH_FILE"
fi

if ! curl -sS "$@" -H "Content-Type: application/json" --data-binary "@$PAYLOAD_FILE" -w '\n__FASTMD_STATUS__:%{http_code}\n' > "$RESPONSE_FILE"; then
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
printf 'Dashboard: %s/dashboard\n' "$BASE_URL" >&2
