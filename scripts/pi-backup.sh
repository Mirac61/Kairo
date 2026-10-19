#!/bin/sh
# Kairo-Daten vom Raspberry Pi auf den Mac sichern (Datenbank + Notizen).
#   PI=<benutzer>@<pi-host> ./scripts/pi-backup.sh             einmal sichern
#   PI=<benutzer>@<pi-host> ./scripts/pi-backup.sh install     täglich um 12:00 per launchd
#   ./scripts/pi-backup.sh uninstall                           launchd-Job entfernen
# ZIEL ist der Ordner auf dem Pi wie bei deploy.sh, BACKUP der Ordner auf dem Mac.
# ssh zum Pi muss ohne Abfrage gehen (Schlüssel im Schlüsselbund), sonst bricht der Lauf ab.
set -eu

LABEL=app.kairo.pi-backup
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
LOG="$HOME/Library/Logs/kairo-pi-backup.log"

case "${1:-}" in
uninstall)
  launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
  rm -f "$PLIST"
  echo "launchd-Job entfernt"
  exit 0
  ;;
esac

PI="${PI:?Benutzer und Adresse angeben, z. B.: PI=<benutzer>@<pi-host> $0}"
ZIEL="${ZIEL:-/home/${PI%@*}/kairo}"
BACKUP="${BACKUP:-$HOME/Backups/kairo-pi}"

if [ "${1:-}" = install ]; then
  SELF=$(cd "$(dirname "$0")" && pwd)/$(basename "$0")
  mkdir -p "$(dirname "$PLIST")" "$(dirname "$LOG")"
  cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>$LABEL</string>
  <key>ProgramArguments</key><array><string>/bin/sh</string><string>$SELF</string></array>
  <key>EnvironmentVariables</key><dict>
    <key>PI</key><string>$PI</string>
    <key>ZIEL</key><string>$ZIEL</string>
    <key>BACKUP</key><string>$BACKUP</string>
    <key>PATH</key><string>/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin</string>
  </dict>
  <key>StartCalendarInterval</key><dict><key>Hour</key><integer>12</integer><key>Minute</key><integer>0</integer></dict>
  <key>StandardOutPath</key><string>$LOG</string>
  <key>StandardErrorPath</key><string>$LOG</string>
</dict></plist>
EOF
  launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
  launchctl bootstrap "gui/$(id -u)" "$PLIST"
  echo "täglich um 12:00 nach $BACKUP, Log: $LOG"
  echo "jetzt testen: launchctl kickstart gui/$(id -u)/$LABEL"
  exit 0
fi

SSH="ssh -o BatchMode=yes -o ConnectTimeout=10"
TMP=/tmp/kairo-pi-backup.db
STAMP=$(date +%Y%m%d-%H%M%S)
mkdir -p "$BACKUP/db" "$BACKUP/notes"
echo "$(date '+%F %T') Sichere $PI:$ZIEL nach $BACKUP"

# Konsistente Kopie per VACUUM INTO (kairo backup), auch während der Dienst läuft.
$SSH "$PI" "KAIRO_DB_PATH='$ZIEL/data/kairo.db' KAIRO_NOTES_DIR='$ZIEL/notes' '$ZIEL/kairo' backup $TMP >/dev/null"
scp -q -o BatchMode=yes "$PI:$TMP" "$BACKUP/db/kairo-$STAMP.db"
$SSH "$PI" "rm -f $TMP"

# Ohne --delete: was auf dem Pi verschwindet, bleibt hier erhalten.
rsync -a -e "$SSH" "$PI:$ZIEL/notes/" "$BACKUP/notes/"

# Die neuesten 30 Datenbank-Backups behalten.
ls -1t "$BACKUP"/db/kairo-*.db | tail -n +31 | while read -r f; do rm -f "$f"; done
echo "$(date '+%F %T') fertig: db/kairo-$STAMP.db"
