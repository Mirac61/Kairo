# Kairo auf dem Raspberry Pi (nur über VPN)

Kairo läuft auf dem Pi als systemd-Dienst und lauscht nur auf der
WireGuard-Adresse `10.8.50.1`. Im Heimnetz oder Internet ist es damit
nicht erreichbar, nur mit verbundenem VPN. Weil Kairo dann nicht an
`127.0.0.1` bindet, ist Login automatisch aktiv. Lokal am Laptop bleibt
Kairo wie bisher ohne Anmeldung.

```text
Laptop/Handy ──WireGuard──▶ Pi wg0 10.8.50.1 ──▶ Kairo :8742 (Login)
```

## Voraussetzung: VPN auf dem Pi

`pi-wireguard.sh` aus dem Smart-Home-Repository (`smart-home-repo`) richtet
`wg0` mit `10.8.50.0/24` ein (eigenes Gerät = `.2`, weiteres Gerät = `.3`). Die Firewall
lässt Kairo nur für das eigene Gerät durch, nicht für andere VPN-Teilnehmer:

```sh
sudo ufw allow in on wg0 from 10.8.50.2 to any port 8742 proto tcp comment 'Kairo nur eigenes Geraet'
```

WireGuard prüft die Absenderadresse pro Peer (`AllowedIPs`), `.3` kann sich
also nicht als `.2` ausgeben. Kairo hat genau ein Konto, ein zweiter Nutzer
bekommt keinen eigenen Bereich.

## Deployen

Vom Laptop aus, im Repository:

```sh
PI=user@raspberrypi.local ./deploy.sh
```

Das Skript prüft die Architektur des Pi (`armv6l`, `armv7l`, `aarch64`)
und ob `wg0` die `10.8.50.1` hat. Danach baut es WebUI und Binary,
kopiert alles nach `~/kairo` und installiert `scripts/kairo.service`
als systemweiten Dienst. Daten liegen in `~/kairo/data`, Notizen in
`~/kairo/notes`. Bei jedem Start legt Kairo ein Backup an.

## Erstes Konto

VPN verbinden und `http://10.8.50.1:8742/setup` öffnen. Das erste Konto
kann jeder anlegen, der Kairo erreicht. Danach ist die Registrierung
geschlossen (`403`), weitere Anmeldungen laufen über `/login`. Passwort:
mindestens 8 Zeichen.

## Extension und Menüleiste gegen den Pi

Beide melden sich mit dem API-Token aus `~/.config/kairo/token` an, nicht
über den Login. Am einfachsten bekommt der Pi dasselbe Token wie der
Laptop, dann funktionieren beide Instanzen ohne Umstellen:

```sh
scp ~/.config/kairo/token user@raspberrypi.local:.config/kairo/token
ssh -t user@raspberrypi.local sudo systemctl restart kairo
```

- Extension: Einstellung `kairo.backendUrl` auf `http://10.8.50.1:8742`.
- Menüleiste: `KAIRO_URL=http://10.8.50.1:8742`.

## Einstellungen

| Variable | Pi-Wert | Bedeutung |
|---|---|---|
| `KAIRO_HOST` | `10.8.50.1` | Bind-Adresse. Bei allem außer `127.0.0.1` ist Login an |
| `KAIRO_AUTH` | – | `true` erzwingt Login auch bei `127.0.0.1`, etwa hinter einem Port-Forward |
| `KAIRO_COOKIE_SECURE` | – | `true` nur hinter einem HTTPS-Reverse-Proxy. Kairo selbst spricht HTTP |
| `KAIRO_SESSION_TTL` | – | Gültigkeit einer Anmeldung, Standard `720h` |

## Fehlersuche

```sh
ssh user@raspberrypi.local
systemctl status kairo
journalctl -u kairo -n 50
ip -4 addr show wg0        # muss 10.8.50.1 zeigen
sudo ufw status | grep 8742
```

- `bind: cannot assign requested address`: `wg0` war beim Start noch nicht
  oben. Der Dienst startet sich wegen `Restart=always` selbst neu.
- Login springt zurück auf `/login`: `KAIRO_COOKIE_SECURE=true` ohne HTTPS.
  Die Variable entfernen.
- Seite lädt nicht: VPN am Gerät verbunden? `ufw`-Regel vorhanden?

## Dashboard

Die Link-Seite (Kairo, Smart Home, Status) läuft nicht auf dem Pi,
sondern als lokale HTML-Datei: `~/code/personal/pi-dashboard/index.html`.
Auf dem Pi ist dafür kein Port offen.
