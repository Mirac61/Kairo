#!/bin/bash
# Kairo in der macOS-Menüleiste (SwiftBar oder xbar): laufender Timer, Pause/Fertig, heutige Tasks starten.
# Installation: brew install --cask swiftbar, dann diese Datei in den SwiftBar-Plugin-Ordner kopieren
# oder verlinken (ln -s). Der Dateiname legt das Intervall fest (10s). Nur Menüleiste, keine Logik:
# Zeit und Zustand kommen vom Backend, Schreibzugriffe laufen mit dem Token aus ~/.config/kairo/token.
URL="${KAIRO_URL:-http://127.0.0.1:8742}"
TOKEN_FILE="${KAIRO_TOKEN_PATH:-$HOME/.config/kairo/token}"
export PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"

if [ -n "$1" ]; then # Aktion: $1 = start|pause|complete, $2 = Task-ID
  curl -s -m 5 -X POST -H "Authorization: Bearer $(cat "$TOKEN_FILE")" "$URL/api/tasks/$2/$1" >/dev/null
  exit 0
fi

today=$(curl -s -m 3 "$URL/api/today") || today=""
if [ -z "$today" ]; then echo "Kairo ○"; echo "---"; echo "Backend nicht erreichbar | color=red"; exit 0; fi

run=$(jq -r '.running_time_entry // empty | "\(.task_id)\t\(.started_at)"' <<<"$today")
if [ -n "$run" ]; then
  id=${run%%$'\t'*}; since=$(date -j -u -f "%Y-%m-%dT%H:%M:%SZ" "${run#*$'\t'}" +%s)
  min=$(( ($(date +%s) - since) / 60 ))
  title=$(curl -s -m 3 "$URL/api/tasks/$id" | jq -r '.title // "Timer"')
  printf '⏱ %d:%02d · %.24s\n---\n' $((min / 60)) $((min % 60)) "${title//|//}"
  echo "Pause | bash=\"$0\" param1=pause param2=$id terminal=false refresh=true"
  echo "Fertig | bash=\"$0\" param1=complete param2=$id terminal=false refresh=true"
else
  printf 'Kairo ▶\n---\n'
  jq -r '.tasks[] | select(.status != "COMPLETED" and .status != "CANCELLED") | "\(.id)\t\(.title)"' <<<"$today" |
    while IFS=$'\t' read -r id title; do
      echo "Start: ${title//|//} | bash=\"$0\" param1=start param2=$id terminal=false refresh=true"
    done
fi
echo "---"
echo "Kairo öffnen | href=$URL"
