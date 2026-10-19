# Kairo auf dem Raspberry Pi (nur über VPN)

Kairo läuft auf dem Pi als systemd-Dienst und lauscht nur auf der
WireGuard-Adresse `<wg-ip>`. Im Heimnetz oder Internet ist es damit
nicht erreichbar, nur mit verbundenem VPN. Weil Kairo dann nicht an
`127.0.0.1` bindet, ist Login automatisch aktiv. Lokal am Laptop bleibt
Kairo wie bisher ohne Anmeldung.

```text
Laptop/Handy ──WireGuard──▶ Pi wg0 <wg-ip> ──▶ Kairo :8742 (Login)
```

## Voraussetzung: VPN auf dem Pi

Richte `wg0` mit deinem WireGuard-Setup ein (z. B. mit einem
WireGuard-Skript aus deinem Netzwerk-Repository). Die Firewall lässt Kairo nur für
das eigene Gerät durch, nicht für andere VPN-Teilnehmer – `<wg-client-ip>`
ist die Adresse deines Notebooks im VPN:

```sh
sudo ufw allow in on wg0 from <wg-client-ip> to any port 8742 proto tcp comment 'Kairo nur eigenes Geraet'
```

WireGuard prüft die Absenderadresse pro Peer (`AllowedIPs`), andere Geräte
können sich also nicht als Adresse eines anderen Peers ausgeben. Kairo hat
genau ein Konto, ein zweiter Nutzer bekommt keinen eigenen Bereich.

## Deployen

Vom Laptop aus, im Repository:

```sh
PI=<benutzer>@<pi-host> ./deploy.sh
```

Das Skript prüft die Architektur des Pi (`armv6l`, `armv7l`, `aarch64`)
und liest die WireGuard-Adresse von `wg0` auf dem Pi aus. Danach baut es
WebUI und Binary, kopiert alles nach `~/kairo` und installiert
`scripts/kairo.service` als systemweiten Dienst. Daten liegen in
`~/kairo/data`, Notizen in `~/kairo/notes`. Bei jedem Start legt Kairo
ein Backup an.

## Erstes Konto

VPN verbinden und `http://<wg-ip>:8742/setup` öffnen. Das erste Konto
kann jeder anlegen, der Kairo erreicht. Danach ist die Registrierung
geschlossen (`403`), weitere Anmeldungen laufen über `/login`. Passwort:
mindestens 8 Zeichen.

## Extension und Menüleiste gegen den Pi

Beide melden sich mit dem API-Token aus `~/.config/kairo/token` an, nicht
über den Login. Am einfachsten bekommt der Pi dasselbe Token wie der
Laptop, dann funktionieren beide Instanzen ohne Umstellen:

```sh
scp ~/.config/kairo/token <benutzer>@<pi-host>:.config/kairo/token
ssh -t <benutzer>@<pi-host> sudo systemctl restart kairo
```

- Extension: Einstellung `kairo.backendUrl` auf `http://<wg-ip>:8742`.
- Menüleiste: `KAIRO_URL=http://<wg-ip>:8742`.

## Backup auf den Mac

Kairo sichert die Datenbank beim Start nach `~/kairo/data/backups`, also
auf dieselbe SD-Karte. `scripts/pi-backup.sh` holt Datenbank und Notizen
auf den Mac nach `~/Backups/kairo-pi` (Time Machine sichert das mit):

```sh
PI=<benutzer>@<pi-host> ./scripts/pi-backup.sh           # einmal
PI=<benutzer>@<pi-host> ./scripts/pi-backup.sh install   # täglich 12:00 (launchd)
./scripts/pi-backup.sh uninstall
```

Die Datenbank kommt als konsistente Kopie (`kairo backup`, geht auch bei
laufendem Dienst) nach `db/`, die neuesten 30 bleiben. Notizen werden nach
`notes/` kopiert, ohne Löschen: was auf dem Pi verschwindet, bleibt hier.
ssh zum Pi muss ohne Abfrage gehen (Schlüssel mit `ssh-add --apple-use-keychain`);
Fehler stehen in `~/Library/Logs/kairo-pi-backup.log`.

Wiederherstellen: Dienst stoppen, Datei hochladen, `kairo restore`, starten.
`kairo restore` prüft, dass die Datei eine SQLite-Datenbank ist und der Port
frei ist (Kairo also nicht läuft). Die bisherige Datenbank samt WAL-Resten
bleibt als `kairo.db.before-restore-<zeit>` daneben liegen.

```sh
ssh -t <benutzer>@<pi-host> sudo systemctl stop kairo
scp ~/Backups/kairo-pi/db/kairo-<datum>.db <benutzer>@<pi-host>:kairo/restore.db
ssh <benutzer>@<pi-host> <kairo-binary> restore kairo/restore.db   # mit denselben KAIRO_*-Variablen wie der Dienst
ssh -t <benutzer>@<pi-host> sudo systemctl start kairo
```

## Einstellungen

| Variable | Pi-Wert | Bedeutung |
|---|---|---|
| `KAIRO_HOST` | `<wg-ip>` | Bind-Adresse. Bei allem außer `127.0.0.1` ist Login an |
| `KAIRO_AUTH` | – | `true` erzwingt Login auch bei `127.0.0.1`, etwa hinter einem Port-Forward |
| `KAIRO_COOKIE_SECURE` | – | `true` nur hinter einem HTTPS-Reverse-Proxy. Kairo selbst spricht HTTP |
| `KAIRO_SESSION_TTL` | – | Gültigkeit einer Anmeldung, Standard `720h` |
| `KAIRO_NTFY_URL` | `http://<wg-ip>:<ntfy-port>` | ntfy-Server (auf dem Pi, nicht ntfy.sh). Mit Topic aktiviert es Erinnerungen |
| `KAIRO_NTFY_TOPIC` | `<topic>` | Langes, zufälliges Topic für die Erinnerungen (Vorlauf `KAIRO_NTFY_LEAD`) |
| `KAIRO_NTFY_LEAD` | `10m` | Vorlauf, mit dem vor Beginn von Aufgaben/Terminen erinnert wird |
| `KAIRO_NTFY_DIGEST` | `08:00` | Uhrzeit der täglichen Meldung „Heute fällig“ mit dem Überfälligen; `off` schaltet sie ab |

## Erinnerungen (ntfy)

Auf dem Pi läuft ein eigener ntfy-Server, nur über WireGuard erreichbar.
Ersetze die Platzhalter durch deine eigenen Werte: `<wg-ip>` ist die Adresse
aus `ip -4 addr show wg0`, `<ntfy-port>` ein freier Port (z. B. `8082`),
`<topic>` ein langer Zufallsname (z. B. `kairo-$(openssl rand -hex 12)`).
Das Topic ist der einzige Schutz innerhalb des VPN, also nicht `kairo` nehmen.

**1. ntfy installieren** (offizielles apt-Repo, auch für armhf):

```sh
sudo mkdir -p /etc/apt/keyrings
sudo curl -fsSL -o /etc/apt/keyrings/ntfy.gpg https://archive.ntfy.sh/apt/keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/ntfy.gpg] https://archive.ntfy.sh/apt stable main" \
  | sudo tee /etc/apt/sources.list.d/ntfy.list
sudo apt update && sudo apt install ntfy
```

**2. `/etc/ntfy/server.yml`:**

```yaml
base-url: "http://<wg-ip>:<ntfy-port>"
listen-http: "<wg-ip>:<ntfy-port>"
# iOS: Sofort-Push bei gesperrtem iPhone. ntfy.sh bekommt nur die Nachrichten-ID
# und einen Hash des Topics und weckt die App über APNs; den Text holt sie vom Pi.
upstream-base-url: "https://ntfy.sh"
cache-file: "/var/cache/ntfy/cache.db"
cache-duration: "24h"
```

Wie Kairo darf ntfy erst nach `wg0` starten:

```sh
sudo mkdir -p /var/cache/ntfy && sudo chown ntfy:ntfy /var/cache/ntfy
sudo systemctl edit ntfy    # Override, dann:
[Unit]
After=wg-quick@wg0.service
[Service]
Restart=always
RestartSec=5
```

**3. Firewall** (nur über das VPN):

```sh
sudo ufw allow in on wg0 to any port <ntfy-port> proto tcp comment 'ntfy ueber VPN'
sudo systemctl enable --now ntfy
curl http://<wg-ip>:<ntfy-port>/v1/health   # {"healthy":true}
```

**4. Kairo anbinden:**

```sh
sudo systemctl edit kairo   # Override, dann:
[Service]
Environment=KAIRO_NTFY_URL=http://<wg-ip>:<ntfy-port>
Environment=KAIRO_NTFY_TOPIC=<topic>
Environment=KAIRO_NTFY_LEAD=10m
sudo systemctl restart kairo
```

**5. Handy:** ntfy-App installieren, VPN an, als Server
`http://<wg-ip>:<ntfy-port>` eintragen und `<topic>` abonnieren. Die URL muss
exakt `base-url` entsprechen, sonst kommt auf iOS kein Push an. Test:

```sh
curl -H "Title: Kairo" -d "Test" http://<wg-ip>:<ntfy-port>/<topic>
```

Das Icon der Benachrichtigung ist immer das der ntfy-App.

Sind `KAIRO_NTFY_URL` und `KAIRO_NTFY_TOPIC` leer, ist die Erinnerung aus.
Die Benachrichtigung kommt als ntfy-Post mit dem Titel als `Title` und dem
Text „Start um HH:MM“ (ggf. plus Ort).

## Fehlersuche

```sh
ssh <benutzer>@<pi-host>
systemctl status kairo
journalctl -u kairo -n 50
ip -4 addr show wg0        # muss die WireGuard-Adresse zeigen
sudo ufw status | grep 8742
```

- `bind: cannot assign requested address`: `wg0` war beim Start noch nicht
  oben. Der Dienst startet sich wegen `Restart=always` selbst neu.
- Login springt zurück auf `/login`: `KAIRO_COOKIE_SECURE=true` ohne HTTPS.
  Die Variable entfernen.
- Seite lädt nicht: VPN am Gerät verbunden? `ufw`-Regel vorhanden?

## Dashboard

Die Link-Seite (Kairo, Smart Home, Status) läuft nicht auf dem Pi,
sondern als lokale HTML-Datei. Auf dem Pi ist dafür kein Port offen.
