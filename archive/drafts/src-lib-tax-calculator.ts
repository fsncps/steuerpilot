// src/lib/tax/calculator.ts
// Steuerberechnungs-Engine für Kanton St. Gallen
// WICHTIG: Alle Beträge in CHF (nicht Rappen) – Rundung auf 1 Franken am Ende

import type {
  Steuerfall,
  Steuerergebnis,
  Steuerparameter,
  Abzuege,
  Einkommen,
  Vermoegen,
  Personalien,
} from './types';

// ============================================================
// Haupt-Berechnungsfunktion
// ============================================================

export function berechneSteuern(
  steuerfall: Steuerfall,
  parameter: Steuerparameter
): Steuerergebnis {
  const { personalien, einkommen, abzuege, vermoegen } = steuerfall;

  // Steuerfüsse ermitteln
  const steuerfussGemeinde = parameter.steuerfuesse.gemeinden[personalien.gemeinde] ?? 100;
  const steuerfussKanton = parameter.steuerfuesse.kanton;
  const steuerfussKirche = ermittleKirchensteuerfuss(personalien.konfession, parameter);

  // 1. Einkommen berechnen
  const totalEinkommen = berechneTotalEinkommen(einkommen);

  // 2. Abzüge berechnen (Kanton)
  const totalAbzuege = berechneTotalAbzuege(abzuege, einkommen, personalien, parameter, false);
  const steuerbaresEinkommen = Math.max(0, Math.round(totalEinkommen - totalAbzuege));

  // 3. Abzüge Bund (können abweichen, z.B. Fahrkosten)
  const totalAbzuegeBund = berechneTotalAbzuege(abzuege, einkommen, personalien, parameter, true);
  const steuerbaresEinkommenBund = Math.max(0, Math.round(totalEinkommen - totalAbzuegeBund));

  // 4. Vermögen
  const totalVermoegen = berechneTotalVermoegen(vermoegen);
  const totalSchulden = vermoegen.schulden;
  const reinvermoegen = Math.max(0, totalVermoegen - totalSchulden);
  const freibetrag = parameter.tarif.vermoegenssteuer.freibetragPerson
    + (personalien.kinder.length * parameter.tarif.vermoegenssteuer.freibetragKind);
  const istVerheiratet = ['verheiratet', 'eingetragene_partnerschaft'].includes(personalien.zivilstand);
  const freibetragTotal = istVerheiratet ? freibetrag * 2 : freibetrag;
  const steuerbaresVermoegen = Math.max(0, reinvermoegen - freibetragTotal);

  // 5. Einfache Steuer (Kanton SG)
  const einfacheSteuer = berechneEinfacheSteuer(
    steuerbaresEinkommen,
    istVerheiratet || personalien.zivilstand === 'geschieden' && personalien.kinder.length > 0,
    parameter
  );

  // 6. Steuerbeträge
  const kantonssteuer = Math.round(einfacheSteuer * steuerfussKanton / 100);
  const gemeindesteuer = Math.round(einfacheSteuer * steuerfussGemeinde / 100);
  const kirchensteuer = Math.round(einfacheSteuer * steuerfussKirche / 100);

  // 7. Bundessteuer
  const bundessteuer = berechneBundessteuer(steuerbaresEinkommenBund, istVerheiratet, parameter);

  // 8. Vermögenssteuer
  const vermoegensSteuerEinfach = Math.round(steuerbaresVermoegen * parameter.tarif.vermoegenssteuer.rate);
  const vermoegensSteuerKanton = Math.round(vermoegensSteuerEinfach * steuerfussKanton / 100);
  const vermoegensSteuerGemeinde = Math.round(vermoegensSteuerEinfach * steuerfussGemeinde / 100);

  // 9. Kinderabzug auf Steuer (SG-spezifisch: Reduktion des Rechnungsbetrags)
  // TODO: Implementieren – der Kanton SG reduziert den Steuerbetrag pro Kind

  // Total
  const totalSteuer = kantonssteuer + gemeindesteuer + kirchensteuer
    + bundessteuer + vermoegensSteuerKanton + vermoegensSteuerGemeinde;

  return {
    totalEinkommen,
    totalAbzuege,
    steuerbaresEinkommen,
    steuerbaresEinkommenBund,
    totalVermoegen,
    totalSchulden,
    steuerbaresVermoegen,
    einfacheSteuer,
    kantonssteuer,
    gemeindesteuer,
    kirchensteuer,
    bundessteuer,
    vermoegensSteuerKanton,
    vermoegensSteuerGemeinde,
    totalSteuer,
    gemeinde: personalien.gemeinde,
    steuerfussGemeinde,
    steuerfussKanton,
    steuerfussKirche,
    steuerperiode: steuerfall.steuerperiode,
  };
}

// ============================================================
// Einkommen
// ============================================================

function berechneTotalEinkommen(einkommen: Einkommen): number {
  let total = 0;

  // Haupterwerb
  total += einkommen.haupterwerb.bruttolohn;

  // Nebenerwerb
  for (const ne of einkommen.nebenerwerb) {
    total += ne.bruttolohn;
  }

  // Kapitalerträge
  total += einkommen.wertschriftenErtraege;
  total += einkommen.bankzinsen;
  total += einkommen.beteiligungsErtraege;

  // Übrige
  total += einkommen.liegenschaftenEinkuenfte;
  total += einkommen.uebrigeEinkuenfte;
  total += einkommen.renten;
  total += einkommen.kinderzulagen;

  return total;
}

// ============================================================
// Abzüge
// ============================================================

function berechneTotalAbzuege(
  abzuege: Abzuege,
  einkommen: Einkommen,
  personalien: Personalien,
  parameter: Steuerparameter,
  isBund: boolean
): number {
  let total = 0;
  const params = parameter.abzuege;
  const istVerheiratet = ['verheiratet', 'eingetragene_partnerschaft'].includes(personalien.zivilstand);

  // Berufskosten
  total += berechneBerufskosten(abzuege.berufskosten, einkommen.haupterwerb.nettolohn, parameter, isBund);

  // Sozialabgaben (AHV/IV/EO/ALV/NBUV – vom Lohnausweis, kein Limit)
  total += abzuege.sozialabgaben;

  // BVG
  total += abzuege.bvgBeitraege;

  // Säule 3a
  const hatPK = abzuege.bvgBeitraege > 0;
  const max3a = hatPK ? params.vorsorge.saeule3aMitPk : Math.min(params.vorsorge.saeule3aOhnePk, einkommen.haupterwerb.nettolohn * params.vorsorge.saeule3aOhnePkProzent);
  total += Math.min(abzuege.saeule3a, max3a);

  // Versicherungsprämien
  const versMax = berechneVersicherungsMax(istVerheiratet, personalien.kinder.length, hatPK, parameter, isBund);
  total += Math.min(abzuege.versicherungspraemien, versMax);

  // Krankheitskosten (über 5% Schwelle)
  const nettoeinkommen = berechneTotalEinkommen(einkommen) - abzuege.sozialabgaben - abzuege.bvgBeitraege;
  const schwelle = nettoeinkommen * params.krankheitskosten.selbstbehaltProzent;
  total += Math.max(0, abzuege.krankheitskosten - schwelle);

  // Schuldzinsen
  const schuldzinsenMax = params.schuldzinsenMaxBasis + einkommen.bankzinsen + einkommen.wertschriftenErtraege;
  total += Math.min(abzuege.schuldzinsen, schuldzinsenMax);

  // Unterhaltsbeiträge
  total += abzuege.unterhaltsbeitraege;

  // Spenden
  const spendenMax = nettoeinkommen * params.spendenMaxProzent;
  total += Math.min(abzuege.spenden, spendenMax);

  // Weiterbildung
  const wbMax = isBund ? parameter.bundessteuer.weiterbildungMax : params.weiterbildungMax;
  total += Math.min(abzuege.weiterbildung, wbMax);

  // Liegenschaftsunterhalt
  total += abzuege.liegenschaftsunterhalt;

  // Sozialabzüge (Kinder)
  for (const kind of personalien.kinder) {
    if (isBund) {
      total += parameter.bundessteuer.kinderabzug;
    } else {
      total += kind.inAusbildung ? params.sozialabzuege.kinderabzugAusbildung : params.sozialabzuege.kinderabzugVorschule;
    }
  }

  return total;
}

function berechneBerufskosten(
  berufskosten: Abzuege['berufskosten'],
  nettolohn: number,
  parameter: Steuerparameter,
  isBund: boolean
): number {
  const params = parameter.abzuege.berufskosten;
  let total = 0;

  // Fahrkosten
  const fahrkostenMax = isBund ? params.fahrkostenMaxBund : params.fahrkostenMax;
  let fahrkosten = 0;

  switch (berufskosten.fahrkosten.art) {
    case 'oev':
      fahrkosten = berufskosten.fahrkosten.oevKosten ?? 0;
      break;
    case 'auto':
      fahrkosten = berufskosten.fahrkosten.distanzKm * 2 * berufskosten.fahrkosten.arbeitstage * params.kmAuto;
      break;
    case 'motorrad':
      fahrkosten = berufskosten.fahrkosten.distanzKm * 2 * berufskosten.fahrkosten.arbeitstage * params.kmMotorrad;
      break;
    case 'velo':
      fahrkosten = params.veloPauschale;
      break;
    case 'keine':
      fahrkosten = 0;
      break;
  }
  total += Math.min(fahrkosten, fahrkostenMax);

  // Verpflegung
  if (berufskosten.verpflegung.auswärtig) {
    if (berufskosten.verpflegung.kantine) {
      total += Math.min(
        berufskosten.verpflegung.arbeitstage * params.verpflegungKantineTag,
        params.verpflegungKantineMax
      );
    } else {
      total += Math.min(
        berufskosten.verpflegung.arbeitstage * params.verpflegungTag,
        params.verpflegungMax
      );
    }
  }

  // Übrige Berufskosten (3% Pauschale)
  if (berufskosten.uebrigeBerufskosten > 0) {
    total += berufskosten.uebrigeBerufskosten;
  } else {
    // Pauschale: 3% vom Nettolohn, min 2000, max 4000
    const pauschale = nettolohn * params.uebrigeProzent;
    total += Math.max(params.uebrigeMin, Math.min(pauschale, params.uebrigeMax));
  }

  // Weiterbildung
  total += Math.min(berufskosten.weiterbildungskosten, parameter.abzuege.weiterbildungMax);

  return total;
}

function berechneVersicherungsMax(
  istVerheiratet: boolean,
  anzahlKinder: number,
  hatVorsorge: boolean,
  parameter: Steuerparameter,
  isBund: boolean
): number {
  if (isBund) {
    const bund = parameter.bundessteuer;
    let max = istVerheiratet ? bund.versicherungenGemeinsam : bund.versicherungenAlleinstehend;
    max += anzahlKinder * bund.versicherungenProKind;
    return max;
  }

  const vers = parameter.abzuege.versicherungen;
  let max: number;
  if (istVerheiratet) {
    max = hatVorsorge ? vers.gemeinsam : vers.gemeinsamOhneVorsorge;
  } else {
    max = hatVorsorge ? vers.alleinstehend : vers.alleinstehendOhneVorsorge;
  }
  max += anzahlKinder * vers.proKind;
  return max;
}

// ============================================================
// Einfache Steuer (Kanton SG – progressiver Tarif)
// ============================================================

function berechneEinfacheSteuer(
  steuerbaresEinkommen: number,
  splitting: boolean,
  parameter: Steuerparameter
): number {
  const tarif = parameter.tarif.einkommenssteuer;
  
  // Bei Splitting: Satz auf halbes Einkommen, dann auf ganzes anwenden
  const satzEinkommen = splitting ? Math.floor(steuerbaresEinkommen / 2) : steuerbaresEinkommen;

  // Maximalsatz-Prüfung
  const maxEinkommen = splitting ? tarif.maxEinkommenGemeinsam : tarif.maxEinkommenAlleinstehend;
  if (steuerbaresEinkommen > maxEinkommen) {
    return Math.round(steuerbaresEinkommen * tarif.maxRate);
  }

  // Tarif-Stufe finden
  const stufen = tarif.stufen;
  let steuerSatz = 0;

  for (const stufe of stufen) {
    if (satzEinkommen >= stufe.von && satzEinkommen <= stufe.bis) {
      // Grenzsteuersatz anwenden
      steuerSatz = stufe.basisSteuer + (satzEinkommen - stufe.von) * stufe.rate;
      break;
    }
  }

  // Bei Splitting: Satz verdoppeln (bzw. auf ganzes Einkommen anwenden)
  if (splitting && satzEinkommen > 0) {
    const effektiverSatz = steuerSatz / satzEinkommen;
    return Math.round(steuerbaresEinkommen * effektiverSatz);
  }

  return Math.round(steuerSatz);
}

// ============================================================
// Bundessteuer
// ============================================================

function berechneBundessteuer(
  steuerbaresEinkommen: number,
  istVerheiratet: boolean,
  parameter: Steuerparameter
): number {
  // Vereinfachte Berechnung – für MVP
  // TODO: Verheirateten-Tarif separat implementieren (Tarif für Verheiratete ist ein eigener Tarif bei der dBSt)
  const stufen = parameter.bundessteuer.stufen_alleinstehend;

  let steuer = 0;
  for (const stufe of stufen) {
    if (steuerbaresEinkommen >= stufe.von && steuerbaresEinkommen <= stufe.bis) {
      steuer = stufe.basisSteuer + (steuerbaresEinkommen - stufe.von) * stufe.rate;
      break;
    }
  }

  // Bei Verheirateten: Tarif ist tiefer (separater Tarif, hier vereinfacht)
  if (istVerheiratet) {
    steuer = steuer * 0.85; // Approximation – muss durch echten Verheirateten-Tarif ersetzt werden
  }

  return Math.round(steuer);
}

// ============================================================
// Vermögen
// ============================================================

function berechneTotalVermoegen(vermoegen: Vermoegen): number {
  let total = 0;

  for (const konto of vermoegen.bankguthaben) {
    total += konto.saldo;
  }

  total += vermoegen.wertschriften;
  total += vermoegen.fahrzeuge;
  total += vermoegen.lebensversicherungRueckkauf;
  total += vermoegen.uebrigesVermoegen;

  return total;
}

// ============================================================
// Kirchensteuer
// ============================================================

function ermittleKirchensteuerfuss(
  konfession: Personalien['konfession'],
  parameter: Steuerparameter
): number {
  const kirche = parameter.steuerfuesse.kirche;
  
  switch (konfession) {
    case 'evangelisch':
      return typeof kirche.evangelisch === 'number' ? kirche.evangelisch : kirche.evangelisch.typisch ?? 24;
    case 'katholisch':
      return typeof kirche.katholisch === 'number' ? kirche.katholisch : kirche.katholisch.typisch ?? 24;
    case 'christkatholisch':
      return typeof kirche.christkatholisch === 'number' ? kirche.christkatholisch : 24;
    default:
      return 0;
  }
}

// ============================================================
// Hilfsfunktionen
// ============================================================

/**
 * Formatiert einen Betrag im Schweizer Format: CHF 1'234.56
 */
export function formatCHF(betrag: number): string {
  const formatted = betrag.toFixed(2).replace(/\B(?=(\d{3})+(?!\d))/g, "'");
  return `CHF ${formatted}`;
}

/**
 * Formatiert einen Betrag ohne Dezimalstellen: CHF 1'234
 */
export function formatCHFRund(betrag: number): string {
  const rounded = Math.round(betrag);
  const formatted = rounded.toString().replace(/\B(?=(\d{3})+(?!\d))/g, "'");
  return `CHF ${formatted}`;
}
