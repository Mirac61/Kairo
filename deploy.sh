#!/bin/sh
# Kairo auf den Raspberry Pi deployen (nur über WireGuard erreichbar, mit Login).
#   PI=user@raspberrypi.local ./deploy.sh
#   PI=user@192.168.99.50 ZIEL=/home/user/kairo-app ./deploy.sh
# Voraussetzung auf dem Pi: wg0 mit 10.8.50.1 läuft (pi-wireguard.sh aus smart-home-repo).
set -eu
cd "$(dirname "$0")"

PI="${PI:?Benutzer und Adresse angeben, z. B.: PI=user@raspberrypi.local ./deploy.sh}"
USER_NAME="${PI%@*}"
ZIEL="${ZIEL:-/home/$USER_NAME/kairo}"
VPN_IP="10.8.50.1"

echo "→ Prüfe Pi ..."
ARCH=$(ssh "$PI" uname -m)
case "$ARCH" in
  armv6l)  GOARCH=arm GOARM=6 ;;
  armv7l)  GOARCH=arm GOARM=7 ;;
  aarch64) GOARCH=arm64 GOARM= ;;
  *) echo "Unbekannte Architektur auf dem Pi: $ARCH" >&2; exit 1 ;;
esac
if ! ssh "$PI" "ip -4 addr show wg0 2>/dev/null | grep -q 'inet $VPN_IP/'"; then
  echo "wg0 mit $VPN_IP läuft auf dem Pi nicht. Erst WireGuard einrichten/starten (sudo wg-quick up wg0)." >&2
  exit 1
fi

echo "→ Baue WebUI ..."
(cd frontend && pnpm build)

echo "→ Baue Kairo für linux/$GOARCH ($ARCH) ..."
mkdir -p dist
(cd backend && go vet ./... && go test ./... \
  && GOOS=linux GOARCH=$GOARCH GOARM=$GOARM CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ../dist/kairo ./cmd/server)
sed -e "s|PI_USER|$USER_NAME|g" -e "s|PI_ZIEL|$ZIEL|g" scripts/kairo.service > dist/kairo.service

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
