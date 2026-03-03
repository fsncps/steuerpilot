# SteuerPilot SG

KI-gestützte Web-App zur Vorbereitung der Steuererklärung für Privatpersonen im Kanton St. Gallen. **GO-VERSION.**

Lohnausweis, Kontoauszug und Säule-3a-Belege hochladen → Claude Vision extrahiert die Daten →
5-Schritt-Wizard → lokale Steuerberechnung → PDF-Export für E-Tax SG.

---

## (Windows)

1. `steuerpilot.exe` herunterladen und starten
2. Browser öffnet sich automatisch
3. Anthropic API-Schlüssel eingeben (einmalig)
4. Fertig

---

## Entwicklung

```bash
make tools        # einmalig: templ + air installieren
cp .env.example .env && vi .env   # ANTHROPIC_API_KEY eintragen
make dev          # templ watch + air hot-reload → http://localhost:3000
```

## Build

```bash
make build          # Linux-Binary
make build-windows  # steuerpilot.exe (Windows AMD64)
make test-calc      # Steuerrechner-Unit-Tests
make test           # alle Tests
```

## Umgebungsvariablen (.env)

| Variable | Pflicht | Default |
|---|---|---|
| `ANTHROPIC_API_KEY` | Dev/Server | — |
| `SESSION_SECRET` | Prod | — |
| `PORT` | nein | `3000` |
| `ENV` | nein | `development` |

Im Windows-Exe-Modus wird kein `.env` benötigt — der Schlüssel wird beim ersten Start über
die Setup-Seite eingegeben und in `%APPDATA%\SteuerPilot\config.json` gespeichert.
