#!/bin/sh
# Kairo auf den Raspberry Pi deployen (nur über WireGuard erreichbar, mit Login).
#   PI=<benutzer>@<pi-host> ./deploy.sh
#   PI=<benutzer>@<pi-host> ZIEL=/home/<benutzer>/kairo-app ./deploy.sh
# Voraussetzung auf dem Pi: Der WireGuard-Tunnel läuft als Interface wg0
# (z. B. eingerichtet durch ein <wireguard-skript>-Skript aus deinem Netzwerk-Repo).
set -eu
cd "$(dirname "$0")"

PI="${PI:?Benutzer und Adresse angeben, z. B.: PI=<benutzer>@<pi-host> ./deploy.sh}"
USER_NAME="${PI%@*}"
ZIEL="${ZIEL:-/home/$USER_NAME/kairo}"

echo "→ Prüfe Pi ..."
ARCH=$(ssh "$PI" uname -m)
case "$ARCH" in
  armv6l)  GOARCH=arm GOARM=6 ;;
  armv7l)  GOARCH=arm GOARM=7 ;;
  aarch64) GOARCH=arm64 GOARM= ;;
  *) echo "Unbekannte Architektur auf dem Pi: $ARCH" >&2; exit 1 ;;
esac
# WireGuard-Adresse des Pi von wg0 ablesen; nichts hartkodiert.
VPN_IP=$(ssh "$PI" "ip -4 addr show wg0 2>/dev/null | awk '/inet /{sub(/\/.*/, \"\", \$2); print \$2; exit}'") || true
if [ -z "$VPN_IP" ]; then
  echo "Auf dem Pi läuft kein wg0 mit Adresse. Erst WireGuard einrichten/starten (sudo wg-quick up wg0)." >&2
  exit 1
fi

echo "→ Baue WebUI ..."
(cd frontend && pnpm build)

echo "→ Baue Kairo für linux/$GOARCH ($ARCH) ..."
mkdir -p dist
(cd backend && go vet ./... && go test ./... \
  && GOOS=linux GOARCH=$GOARCH GOARM=$GOARM CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ../dist/kairo ./cmd/server)
sed -e "s|PI_USER|$USER_NAME|g" -e "s|PI_ZIEL|$ZIEL|g" -e "s|PI_VPN|$VPN_IP|g" scripts/kairo.service > dist/kairo.service

echo "→ Kopiere auf den Pi ..."
ssh "$PI" "mkdir -p '$ZIEL/data' '$ZIEL/notes'"
scp -q dist/kairo "$PI:$ZIEL/kairo.neu"
scp -q dist/kairo.service "$PI:/tmp/kairo.service"

echo "→ Aktiviere Dienst (sudo-Passwort des Pi) ..."
ssh -t "$PI" "mv '$ZIEL/kairo.neu' '$ZIEL/kairo' \
  && sudo mv /tmp/kairo.service /etc/systemd/system/kairo.service \
  && sudo systemctl daemon-reload \
  && sudo systemctl enable --now kairo \
  && sudo systemctl restart kairo \
  && sleep 2 && systemctl is-active kairo \
  || { sudo journalctl -u kairo -n 20 --no-pager; exit 1; }"

echo ""
echo "fertig: http://$VPN_IP:8742 (nur mit verbundenem VPN)"
echo "Erstes Konto anlegen: http://$VPN_IP:8742/setup"
