// src/lib/tax/types.ts
// Zentrale TypeScript-Typen für alle Steuerdaten

// ============================================================
// Personalien
// ============================================================

export type Zivilstand = 'alleinstehend' | 'verheiratet' | 'geschieden' | 'verwitwet' | 'getrennt' | 'eingetragene_partnerschaft';

export type Konfession = 'evangelisch' | 'katholisch' | 'christkatholisch' | 'andere' | 'keine';

export interface Personalien {
  vorname: string;
  nachname: string;
  geburtsdatum: string;               // ISO date
  zivilstand: Zivilstand;
  konfession: Konfession;
  gemeinde: string;                    // SG Gemeinde
  kinder: Kind[];
  partner?: Personalien;               // Bei Verheirateten
}

export interface Kind {
  vorname: string;
  geburtsdatum: string;
  inAusbildung: boolean;
  fremdbetreuung: boolean;
  betreuungskosten?: number;
}

// ============================================================
// Einkommen
// ============================================================

export interface Einkommen {
  haupterwerb: Erwerbseinkommen;
  nebenerwerb: Erwerbseinkommen[];
  wertschriftenErtraege: number;       // Ziff. 4
  bankzinsen: number;                  // Ziff. 4.1
  beteiligungsErtraege: number;        // Ziff. 4.3
  liegenschaftenEinkuenfte: number;    // Ziff. 5 (nicht im MVP)
  uebrigeEinkuenfte: number;           // Ziff. 6
  renten: number;                      // Ziff. 7
  kinderzulagen: number;               // Ziff. 5 Lohnausweis
}

export interface Erwerbseinkommen {
  arbeitgeber: string;
  bruttolohn: number;                  // Ziff. 8
  nettolohn: number;                   // Ziff. 11
  ahvIvEoAlvNbuv: number;             // Ziff. 9
  bvgOrdentlich: number;              // Ziff. 10.1
  bvgEinkauf: number;                 // Ziff. 10.2
  quellensteuer: number;              // Ziff. 12
  spesenEffektiv: number;             // Ziff. 13.1
  spesenPauschal: number;             // Ziff. 13.2
  aussendienstProzent: number;        // Ziff. 15
  hatGeschaeftsauto: boolean;         // Feld F
  hatGA: boolean;                     // Feld F
  hatKantine: boolean;                // Feld G
}

// ============================================================
// Berufskosten (Formular 4)
// ============================================================

export interface Berufskosten {
  fahrkosten: Fahrkosten;
  verpflegung: Verpflegung;
  uebrigeBerufskosten: number;        // Berechnet oder manuell
  weiterbildungskosten: number;
}

export interface Fahrkosten {
  art: 'oev' | 'auto' | 'motorrad' | 'velo' | 'keine';
  distanzKm: number;                  // Einfacher Weg
  arbeitstage: number;                // Standardmässig 220
  oevKosten?: number;                 // Effektive ÖV-Kosten
}

export interface Verpflegung {
  auswärtig: boolean;
  kantine: boolean;
  arbeitstage: number;
}

// ============================================================
// Abzüge
// ============================================================

export interface Abzuege {
  berufskosten: Berufskosten;
  sozialabgaben: number;              // AHV/IV etc. (vom Lohnausweis)
  bvgBeitraege: number;              // Ordentlich + Einkauf
  saeule3a: number;
  versicherungspraemien: number;      // Formular 6
  krankheitskosten: number;           // Effektiv bezahlt, nicht erstattet
  schuldzinsen: number;
  unterhaltsbeitraege: number;        // Alimente
  spenden: number;
  weiterbildung: number;
  liegenschaftsunterhalt: number;     // Nicht im MVP
}

// ============================================================
// Vermögen
// ============================================================

export interface Vermoegen {
  bankguthaben: Bankkonto[];
  wertschriften: number;              // Steuerwert per 31.12.
  fahrzeuge: number;
  lebensversicherungRueckkauf: number;
  uebrigesVermoegen: number;
  schulden: number;                   // Hypotheken, Darlehen etc.
}

export interface Bankkonto {
  bank: string;
  bezeichnung: string;
  iban?: string;
  saldo: number;                      // Per 31.12.
  waehrung: string;
  zinsertrag: number;
  verrechnungssteuer: number;
}

// ============================================================
// Berechnungsergebnis
// ============================================================

export interface Steuerergebnis {
  // Einkommen
  totalEinkommen: number;
  totalAbzuege: number;
  steuerbar esEinkommen: number;
  steuerbaresEinkommenBund: number;   // Kann abweichen (z.B. Fahrkosten)

  // Vermögen
  totalVermoegen: number;
  totalSchulden: number;
  steuerbaresVermoegen: number;

  // Steuern Kanton
  einfacheSteuer: number;
  kantonssteuer: number;
  gemeindesteuer: number;
  kirchensteuer: number;

  // Steuern Bund
  bundessteuer: number;

  // Vermögenssteuer
  vermoegensSteuerKanton: number;
  vermoegensSteuerGemeinde: number;

  // Total
  totalSteuer: number;

  // Meta
  gemeinde: string;
  steuerfussGemeinde: number;
  steuerfussKanton: number;
  steuerfussKirche: number;
  steuerperiode: number;
}

// ============================================================
// Optimierungsvorschlag
// ============================================================

export interface Optimierung {
  titel: string;
  beschreibung: string;
  sparpotenzialMin: number | null;
  sparpotenzialMax: number | null;
  aufwand: 'gering' | 'mittel' | 'hoch';
  zeitrahmen: 'sofort' | 'naechstes_jahr' | 'langfristig';
  kategorie: 'vorsorge' | 'berufskosten' | 'versicherung' | 'timing' | 'spenden' | 'sonstiges';
  gesetzlicheGrundlage: string;
}

// ============================================================
// Dokumenten-Extraktion
// ============================================================

export interface ExtraktionsergebnisLohnausweis {
  arbeitgeberName: string;
  arbeitgeberOrt: string;
  arbeitnehmerName: string;
  ahvNummer: string | null;
  bruttolohn: number;
  nettolohn: number;
  sozialabgaben: number;
  bvgOrdentlich: number | null;
  bvgEinkauf: number | null;
  spesenEffektiv: number | null;
  spesenPauschal: number | null;
  aussendienstProzent: number | null;
  hatGeschaeftsauto: boolean;
  hatGA: boolean;
  hatKantine: boolean;
  kinderzulagen: number | null;
  konfidenz: {
    gesamt: 'hoch' | 'mittel' | 'tief';
    unsichereFelder: string[];
  };
}

export interface ExtraktionsergebnisKonto {
  stichtag: string;
  konten: {
    bank: string;
    kontonummer: string | null;
    iban: string | null;
    bezeichnung: string;
    waehrung: string;
    saldo: number;
    zinsertrag: number | null;
    verrechnungssteuer: number | null;
  }[];
  konfidenz: {
    gesamt: 'hoch' | 'mittel' | 'tief';
    unsichereFelder: string[];
  };
}

export interface Extraktion3a {
  institut: string;
  steuerjahr: number;
  einzahlung: number;
  art: 'bankkonto' | 'versicherung' | 'wertschriften';
  saldoJahresende: number | null;
  konfidenz: {
    gesamt: 'hoch' | 'mittel' | 'tief';
    unsichereFelder: string[];
  };
}

// ============================================================
// Gesamter Steuerfall
// ============================================================

export interface Steuerfall {
  steuerperiode: number;
  personalien: Personalien;
  einkommen: Einkommen;
  abzuege: Abzuege;
  vermoegen: Vermoegen;
  ergebnis?: Steuerergebnis;
  optimierungen?: Optimierung[];
}

// ============================================================
// Steuerparameter (aus der Datenbank)
// ============================================================

export interface Steuerparameter {
  steuerperiode: number;
  kanton: string;
  tarif: {
    einkommenssteuer: TarifStufe[];
    maxRate: number;
    maxEinkommenGemeinsam: number;
    maxEinkommenAlleinstehend: number;
  };
  vermoegenssteuer: {
    rate: number;
    freibetragPerson: number;
    freibetragKind: number;
  };
  steuerfuesse: {
    kanton: number;
    gemeinden: Record<string, number>;
    kirche: Record<string, number | { min: number; max: number }>;
  };
  abzuege: AbzugsParameter;
  bundessteuer: BundessteuerParameter;
}

export interface TarifStufe {
  von: number;
  bis: number;
  rate: number;           // Grenzsteuersatz
  basisSteuer: number;    // Steuer bis zur Untergrenze
}

export interface AbzugsParameter {
  fahrkostenMax: number;
  fahrkostenMaxBund: number;
  kmAuto: number;
  kmMotorrad: number;
  veloPauschale: number;
  verpflegungTag: number;
  verpflegungMax: number;
  verpflegungKantineTag: number;
  verpflegungKantineMax: number;
  uebrigeMin: number;
  uebrigeMax: number;
  uebrigeProzent: number;
  saeule3aMitPk: number;
  saeule3aOhnePk: number;
  saeule3aOhnePkProzent: number;
  versicherungenAlleinstehend: number;
  versicherungenAlleinstehendOhneVorsorge: number;
  versicherungenGemeinsam: number;
  versicherungenGemeinsamOhneVorsorge: number;
  versicherungenProKind: number;
  krankheitskostenSelbstbehalt: number;
  weiterbildungMax: number;
  schuldzinsenMaxBasis: number;
  spendenMaxProzent: number;
  kinderabzugVorschule: number;
  kinderabzugAusbildung: number;
}

export interface BundessteuerParameter {
  tarif: TarifStufe[];
  maxRate: number;
  fahrkostenMax: number;
  versicherungenAlleinstehend: number;
  versicherungenGemeinsam: number;
  versicherungenProKind: number;
  kinderabzug: number;
  weiterbildungMax: number;
}
