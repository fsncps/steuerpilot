// src/lib/anthropic.ts
// Anthropic API Client für SteuerPilot SG
// Verwendet für: Dokumenten-Extraktion (Vision) und Optimierungsvorschläge

import Anthropic from '@anthropic-ai/sdk';
import type {
  ExtraktionsergebnisLohnausweis,
  ExtraktionsergebnisKonto,
  Extraktion3a,
  Optimierung,
  Steuerfall,
} from './tax/types';

// ============================================================
// Client
// ============================================================

const client = new Anthropic({
  apiKey: process.env.ANTHROPIC_API_KEY!,
});

const MODEL = 'claude-sonnet-4-5-20250929';

// ============================================================
// Dokumenten-Extraktion
// ============================================================

const LOHNAUSWEIS_PROMPT = `Du analysierst einen Schweizer Lohnausweis (Formular 11 / Neues Lohnausweisformular NLA).
Extrahiere alle steuerlich relevanten Felder.

Antworte NUR mit einem JSON-Objekt, kein Fliesstext:

{
  "arbeitgeber_name": string,
  "arbeitgeber_ort": string,
  "arbeitnehmer_name": string,
  "ahv_nummer": string | null,
  "ziff8_bruttolohn": number,
  "ziff9_sozialabgaben": number,
  "ziff10_1_bvg_ordentlich": number | null,
  "ziff10_2_bvg_einkauf": number | null,
  "ziff11_nettolohn": number,
  "ziff12_quellensteuer": number | null,
  "ziff13_1_spesen_effektiv": number | null,
  "ziff13_2_spesen_pauschal": number | null,
  "ziff15_aussendienst_prozent": number | null,
  "ziff5_kinderzulagen": number | null,
  "feld_f_ga_oder_geschaeftsauto": boolean,
  "feld_g_kantine": boolean,
  "konfidenz": {
    "gesamt": "hoch" | "mittel" | "tief",
    "unsichere_felder": string[]
  }
}

Regeln:
- Beträge in CHF als Zahl (z.B. 85000.00), kein Tausendertrenner
- null für fehlende/unleserliche Felder
- Nicht raten – lieber null als falscher Wert
- Wenn das Bild kein Lohnausweis ist: {"error": "Kein Lohnausweis erkannt"}`;

const KONTOAUSZUG_PROMPT = `Du analysierst einen Schweizer Bankauszug oder eine Kontoübersicht.
Extrahiere die steuerlich relevanten Daten.

Antworte NUR mit JSON:

{
  "stichtag": string,
  "konten": [
    {
      "bank": string,
      "kontonummer": string | null,
      "iban": string | null,
      "bezeichnung": string,
      "waehrung": string,
      "saldo": number,
      "zinsertrag": number | null,
      "verrechnungssteuer": number | null
    }
  ],
  "konfidenz": {
    "gesamt": "hoch" | "mittel" | "tief",
    "unsichere_felder": string[]
  }
}`;

const SAEULE3A_PROMPT = `Du analysierst eine Schweizer Säule-3a-Einzahlungsbescheinigung.

Antworte NUR mit JSON:

{
  "institut": string,
  "steuerjahr": number,
  "einzahlung": number,
  "art": "bankkonto" | "versicherung" | "wertschriften",
  "saldo_jahresende": number | null,
  "konfidenz": {
    "gesamt": "hoch" | "mittel" | "tief",
    "unsichere_felder": string[]
  }
}`;

export type DocumentType = 'lohnausweis' | 'kontoauszug' | '3a';

function getPrompt(type: DocumentType): string {
  switch (type) {
    case 'lohnausweis': return LOHNAUSWEIS_PROMPT;
    case 'kontoauszug': return KONTOAUSZUG_PROMPT;
    case '3a': return SAEULE3A_PROMPT;
  }
}

export async function extractDocument(
  base64Image: string,
  mediaType: 'image/jpeg' | 'image/png' | 'image/webp' | 'application/pdf',
  type: DocumentType
): Promise<ExtraktionsergebnisLohnausweis | ExtraktionsergebnisKonto | Extraktion3a> {
  const prompt = getPrompt(type);

  const content: Anthropic.MessageCreateParams['messages'][0]['content'] = [
    {
      type: mediaType === 'application/pdf' ? 'document' : 'image',
      source: {
        type: 'base64',
        media_type: mediaType,
        data: base64Image,
      },
    } as any,
    { type: 'text', text: prompt },
  ];

  const response = await client.messages.create({
    model: MODEL,
    max_tokens: 2000,
    messages: [{ role: 'user', content }],
  });

  const text = response.content
    .filter((block) => block.type === 'text')
    .map((block) => (block as Anthropic.TextBlock).text)
    .join('');

  // JSON aus der Antwort extrahieren (falls mit ```json umschlossen)
  const jsonMatch = text.match(/```json\s*([\s\S]*?)```/) || [null, text];
  const jsonStr = (jsonMatch[1] ?? text).trim();

  try {
    return JSON.parse(jsonStr);
  } catch (e) {
    throw new Error(`Konnte JSON nicht parsen: ${jsonStr.substring(0, 200)}`);
  }
}

// ============================================================
// Optimierungsvorschläge
// ============================================================

const OPTIMIERUNG_SYSTEM = `Du bist ein erfahrener Schweizer Steuerberater, spezialisiert auf den Kanton St. Gallen.
Analysiere die Steuersituation und gib konkrete, legale Optimierungsvorschläge.

Regeln:
1. Nur LEGALE Optimierungen
2. Konkrete CHF-Beträge wo möglich
3. Max. 5 Vorschläge, nach Sparpotenzial sortiert
4. Schweizer Hochdeutsch (kein ß)
5. Einfach verständlich

Antworte NUR mit einem JSON-Array:
[
  {
    "titel": string,
    "beschreibung": string,
    "sparpotenzial_min": number | null,
    "sparpotenzial_max": number | null,
    "aufwand": "gering" | "mittel" | "hoch",
    "zeitrahmen": "sofort" | "naechstes_jahr" | "langfristig",
    "kategorie": "vorsorge" | "berufskosten" | "versicherung" | "timing" | "spenden" | "sonstiges",
    "gesetzliche_grundlage": string
  }
]`;

export async function getOptimierungen(steuerfall: Steuerfall): Promise<Optimierung[]> {
  const response = await client.messages.create({
    model: MODEL,
    max_tokens: 2000,
    system: OPTIMIERUNG_SYSTEM,
    messages: [{
      role: 'user',
      content: `Steuerdaten (Kanton SG, Steuerperiode ${steuerfall.steuerperiode}):\n${JSON.stringify(steuerfall, null, 2)}`,
    }],
  });

  const text = response.content
    .filter((block) => block.type === 'text')
    .map((block) => (block as Anthropic.TextBlock).text)
    .join('');

  const jsonMatch = text.match(/```json\s*([\s\S]*?)```/) || [null, text];
  const jsonStr = (jsonMatch[1] ?? text).trim();

  try {
    const raw = JSON.parse(jsonStr);
    return raw.map((item: any) => ({
      titel: item.titel,
      beschreibung: item.beschreibung,
      sparpotenzialMin: item.sparpotenzial_min,
      sparpotenzialMax: item.sparpotenzial_max,
      aufwand: item.aufwand,
      zeitrahmen: item.zeitrahmen,
      kategorie: item.kategorie,
      gesetzlicheGrundlage: item.gesetzliche_grundlage,
    }));
  } catch (e) {
    console.error('Optimierungen JSON Parse Fehler:', e);
    return [];
  }
}
