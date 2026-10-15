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

Richte `wg0` mit deinem WireGuard-Setup ein (z. B. ein `<wireguard-skript>`-
Skript aus deinem Netzwerk-Repository). Die Firewall lässt Kairo nur für
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

## Einstellungen

| Variable | Pi-Wert | Bedeutung |
|---|---|---|
| `KAIRO_HOST` | `<wg-ip>` | Bind-Adresse. Bei allem außer `127.0.0.1` ist Login an |
| `KAIRO_AUTH` | – | `true` erzwingt Login auch bei `127.0.0.1`, etwa hinter einem Port-Forward |
| `KAIRO_COOKIE_SECURE` | – | `true` nur hinter einem HTTPS-Reverse-Proxy. Kairo selbst spricht HTTP |
| `KAIRO_SESSION_TTL` | – | Gültigkeit einer Anmeldung, Standard `720h` |

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
