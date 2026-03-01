package tax

import (
	"math"

	"steuerpilot-go/internal/models"
)

// BerechneSteuern is the core pure function: Steuerfall → Steuerergebnis.
// All amounts in CHF (not Rappen); final values rounded to whole francs.
// Algorithm follows SPEC.md §7 and metamorphosis.md §5 Phase 1.
func BerechneSteuern(sf models.Steuerfall, p models.SteuerparameterDB) models.Steuerergebnis {
	pers := sf.Personalien
	einkomm := sf.Einkommen
	abz := sf.Abzuege
	verm := sf.Vermoegen

	// ── Steuerfüsse ───────────────────────────────────────────────────────────
	steuerfussGemeinde := p.Steuerfuesse.Gemeinden[pers.Gemeinde]
	if steuerfussGemeinde == 0 {
		steuerfussGemeinde = 100
	}
	steuerfussKanton := p.Steuerfuesse.Kanton
	steuerfussKirche := kirchensteuerfuss(pers.Konfession, p)

	istVerheiratet := pers.Zivilstand == models.ZivilstandVerheiratet ||
		pers.Zivilstand == models.ZivilstandEingetragenePartnerschaft
	// Splitting also applies to divorced with children (SG practice)
	splitting := istVerheiratet ||
		(pers.Zivilstand == models.ZivilstandGeschieden && len(pers.Kinder) > 0)

	// ── 1. Einkommen ──────────────────────────────────────────────────────────
	totalEinkommen := berechneTotalEinkommen(einkomm)

	// ── 2. Abzüge Kanton ──────────────────────────────────────────────────────
	totalAbzuege := berechneTotalAbzuege(abz, einkomm, pers, p, false)
	steuerbaresEinkommen := math.Round(math.Max(0, totalEinkommen-totalAbzuege))

	// ── 3. Abzüge Bund (abweichende Limits) ───────────────────────────────────
	totalAbzuegeBund := berechneTotalAbzuege(abz, einkomm, pers, p, true)
	steuerbaresEinkommenBund := math.Round(math.Max(0, totalEinkommen-totalAbzuegeBund))

	// ── 4. Vermögen ───────────────────────────────────────────────────────────
	totalVermoegen := berechneTotalVermoegen(verm)
	totalSchulden := verm.Schulden
	reinvermoegen := math.Max(0, totalVermoegen-totalSchulden)

	freibetrag := p.Tarif.Vermoegenssteuer.FreibetragPerson +
		float64(len(pers.Kinder))*p.Tarif.Vermoegenssteuer.FreibetragKind
	if istVerheiratet {
		freibetrag *= 2
	}
	steuerbaresVermoegen := math.Max(0, reinvermoegen-freibetrag)

	// ── 5. Einfache Steuer ────────────────────────────────────────────────────
	einfacheSteuer := berechneEinfacheSteuer(steuerbaresEinkommen, splitting, p)

	// ── 6. Steuerbeträge ──────────────────────────────────────────────────────
	kantonssteuer := math.Round(einfacheSteuer * float64(steuerfussKanton) / 100)
	gemeindesteuer := math.Round(einfacheSteuer * float64(steuerfussGemeinde) / 100)
	kirchensteuer := math.Round(einfacheSteuer * float64(steuerfussKirche) / 100)

	// ── 7. Bundessteuer ───────────────────────────────────────────────────────
	bundessteuer := berechneBundessteuer(steuerbaresEinkommenBund, istVerheiratet, p)

	// ── 8. Vermögenssteuer ────────────────────────────────────────────────────
	vermoegensSteuerEinfach := math.Round(steuerbaresVermoegen * p.Tarif.Vermoegenssteuer.Rate)
	vermoegensSteuerKanton := math.Round(vermoegensSteuerEinfach * float64(steuerfussKanton) / 100)
	vermoegensSteuerGemeinde := math.Round(vermoegensSteuerEinfach * float64(steuerfussGemeinde) / 100)

	// ── 9. TODO: Kinderabzug auf Steuerbetrag (SG-spezifisch) ─────────────────
	// Kanton SG reduziert den Rechnungsbetrag pro Kind.

	totalSteuer := kantonssteuer + gemeindesteuer + kirchensteuer +
		bundessteuer + vermoegensSteuerKanton + vermoegensSteuerGemeinde

	return models.Steuerergebnis{
		TotalEinkommen:           totalEinkommen,
		TotalAbzuege:             totalAbzuege,
		SteuerbaresEinkommen:     steuerbaresEinkommen,
		SteuerbaresEinkommenBund: steuerbaresEinkommenBund,
		TotalVermoegen:           totalVermoegen,
		TotalSchulden:            totalSchulden,
		SteuerbaresVermoegen:     steuerbaresVermoegen,
		EinfacheSteuer:           einfacheSteuer,
		Kantonssteuer:            kantonssteuer,
		Gemeindesteuer:           gemeindesteuer,
		Kirchensteuer:            kirchensteuer,
		Bundessteuer:             bundessteuer,
		VermoegensSteuerKanton:   vermoegensSteuerKanton,
		VermoegensSteuerGemeinde: vermoegensSteuerGemeinde,
		TotalSteuer:              totalSteuer,
		Gemeinde:                 pers.Gemeinde,
		SteuerfussGemeinde:       steuerfussGemeinde,
		SteuerfussKanton:         steuerfussKanton,
		SteuerfussKirche:         steuerfussKirche,
		Steuerperiode:            sf.Steuerperiode,
	}
}

// ── Einkommen ─────────────────────────────────────────────────────────────────

func berechneTotalEinkommen(e models.Einkommen) float64 {
	total := e.Haupterwerb.Bruttolohn
	for _, ne := range e.Nebenerwerb {
		total += ne.Bruttolohn
	}
	total += e.WertschriftenErtraege
	total += e.Bankzinsen
	total += e.BeteiligungsErtraege
	total += e.LiegenschaftenEinkuenfte
	total += e.UebrigeEinkuenfte
	total += e.Renten
	total += e.Kinderzulagen
	return total
}

// ── Abzüge ────────────────────────────────────────────────────────────────────

func berechneTotalAbzuege(
	abz models.Abzuege,
	einkomm models.Einkommen,
	pers models.Personalien,
	p models.SteuerparameterDB,
	isBund bool,
) float64 {
	cfg := p.Abzuege
	istVerheiratet := pers.Zivilstand == models.ZivilstandVerheiratet ||
		pers.Zivilstand == models.ZivilstandEingetragenePartnerschaft

	total := berechneBerufskosten(abz.Berufskosten, einkomm.Haupterwerb.Nettolohn, p, isBund)

	// Sozialabgaben AHV/IV/EO/ALV/NBUV — kein Limit
	total += abz.Sozialabgaben

	// BVG
	total += abz.BvgBeitraege

	// Säule 3a
	hatPK := abz.BvgBeitraege > 0
	var max3a float64
	if hatPK {
		max3a = cfg.Vorsorge.Saeule3aMitPk
	} else {
		max3a = math.Min(cfg.Vorsorge.Saeule3aOhnePk,
			einkomm.Haupterwerb.Nettolohn*cfg.Vorsorge.Saeule3aOhnePkProzent)
	}
	total += math.Min(abz.Saeule3a, max3a)

	// Versicherungsprämien
	versMax := versicherungsMax(istVerheiratet, len(pers.Kinder), hatPK, p, isBund)
	total += math.Min(abz.Versicherungspraemien, versMax)

	// Krankheitskosten (5%-Schwelle)
	nettoeinkommen := berechneTotalEinkommen(einkomm) - abz.Sozialabgaben - abz.BvgBeitraege
	schwelle := nettoeinkommen * cfg.Krankheitskosten.SelbstbehaltProzent
	total += math.Max(0, abz.Krankheitskosten-schwelle)

	// Schuldzinsen
	schuldzinsenMax := cfg.SchuldzinsenMaxBasis + einkomm.Bankzinsen + einkomm.WertschriftenErtraege
	total += math.Min(abz.Schuldzinsen, schuldzinsenMax)

	// Unterhaltsbeiträge — kein Limit
	total += abz.Unterhaltsbeitraege

	// Spenden (Prozent-Limit vom Nettoeinkommen)
	spendenMax := nettoeinkommen * cfg.SpendenMaxProzent
	total += math.Min(abz.Spenden, spendenMax)

	// Weiterbildung
	wbMax := cfg.WeiterbildungMax
	if isBund {
		wbMax = p.Bundessteuer.WeiterbildungMax
	}
	total += math.Min(abz.Weiterbildung, wbMax)

	// Liegenschaftsunterhalt
	total += abz.Liegenschaftsunterhalt

	// Sozialabzüge Kinder
	for _, kind := range pers.Kinder {
		if isBund {
			total += p.Bundessteuer.Kinderabzug
		} else if kind.InAusbildung {
			total += cfg.Sozialabzuege.KinderabzugAusbildung
		} else {
			total += cfg.Sozialabzuege.KinderabzugVorschule
		}
	}

	return total
}

func berechneBerufskosten(bk models.Berufskosten, nettolohn float64, p models.SteuerparameterDB, isBund bool) float64 {
	cfg := p.Abzuege.Berufskosten
	total := 0.0

	// Fahrkosten
	fahrkostenMax := cfg.FahrkostenMax
	if isBund {
		fahrkostenMax = cfg.FahrkostenMaxBund
	}
	var fahrkosten float64
	switch bk.Fahrkosten.Art {
	case models.FahrkostenOev:
		fahrkosten = bk.Fahrkosten.OevKosten
	case models.FahrkostenAuto:
		fahrkosten = bk.Fahrkosten.DistanzKm * 2 * float64(bk.Fahrkosten.Arbeitstage) * cfg.KmAuto
	case models.FahrkostenMotorrad:
		fahrkosten = bk.Fahrkosten.DistanzKm * 2 * float64(bk.Fahrkosten.Arbeitstage) * cfg.KmMotorrad
	case models.FahrkostenVelo:
		fahrkosten = cfg.VeloPauschale
	}
	total += math.Min(fahrkosten, fahrkostenMax)

	// Verpflegung
	if bk.Verpflegung.Auswaertig {
		if bk.Verpflegung.Kantine {
			total += math.Min(float64(bk.Verpflegung.Arbeitstage)*cfg.VerpflegungKantineTag, cfg.VerpflegungKantineMax)
		} else {
			total += math.Min(float64(bk.Verpflegung.Arbeitstage)*cfg.VerpflegungTag, cfg.VerpflegungMax)
		}
	}

	// Übrige Berufskosten: effektiv oder Pauschale (3% Nettolohn, min 2'000, max 4'000)
	if bk.UebrigeBerufskosten > 0 {
		total += bk.UebrigeBerufskosten
	} else {
		pauschale := nettolohn * cfg.UebrigeProzent
		total += math.Max(cfg.UebrigeMin, math.Min(pauschale, cfg.UebrigeMax))
	}

	// Weiterbildung (Berufskosten-Teil)
	total += math.Min(bk.Weiterbildungskosten, p.Abzuege.WeiterbildungMax)

	return total
}

func versicherungsMax(istVerheiratet bool, anzahlKinder int, hatVorsorge bool, p models.SteuerparameterDB, isBund bool) float64 {
	if isBund {
		b := p.Bundessteuer
		if istVerheiratet {
			return b.VersicherungenGemeinsam + float64(anzahlKinder)*b.VersicherungenProKind
		}
		return b.VersicherungenAlleinstehend + float64(anzahlKinder)*b.VersicherungenProKind
	}

	v := p.Abzuege.Versicherungen
	var base float64
	switch {
	case istVerheiratet && hatVorsorge:
		base = v.Gemeinsam
	case istVerheiratet:
		base = v.GemeinsamOhneVorsorge
	case hatVorsorge:
		base = v.Alleinstehend
	default:
		base = v.AlleinstehendOhneVorsorge
	}
	return base + float64(anzahlKinder)*v.ProKind
}

// ── Einfache Steuer (progressiver Tarif SG) ───────────────────────────────────

func berechneEinfacheSteuer(steuerbaresEinkommen float64, splitting bool, p models.SteuerparameterDB) float64 {
	tarif := p.Tarif.Einkommenssteuer

	// Maximalsatz-Grenze
	maxEinkommen := tarif.MaxEinkommenAlleinstehend
	if splitting {
		maxEinkommen = tarif.MaxEinkommenGemeinsam
	}
	if steuerbaresEinkommen > maxEinkommen {
		return math.Round(steuerbaresEinkommen * tarif.MaxRate)
	}

	// Bei Splitting: Satz auf halbes Einkommen ermitteln, dann auf ganzes anwenden
	satzEinkommen := steuerbaresEinkommen
	if splitting {
		satzEinkommen = math.Floor(steuerbaresEinkommen / 2)
	}

	var steuerSatz float64
	for _, stufe := range tarif.Stufen {
		if satzEinkommen >= stufe.Von && satzEinkommen <= stufe.Bis {
			steuerSatz = stufe.BasisSteuer + (satzEinkommen-stufe.Von)*stufe.Rate
			break
		}
	}

	if splitting && satzEinkommen > 0 {
		effektiverSatz := steuerSatz / satzEinkommen
		return math.Round(steuerbaresEinkommen * effektiverSatz)
	}

	return math.Round(steuerSatz)
}

// ── Bundessteuer ──────────────────────────────────────────────────────────────

// berechneBundessteuer uses the Alleinstehend tariff with a 0.85 factor for
// married couples. TODO: replace with actual Tarif B (dBSt) once added to params.
func berechneBundessteuer(steuerbaresEinkommen float64, istVerheiratet bool, p models.SteuerparameterDB) float64 {
	var steuer float64
	for _, stufe := range p.Bundessteuer.StufenAlleinstehend {
		if steuerbaresEinkommen >= stufe.Von && steuerbaresEinkommen <= stufe.Bis {
			steuer = stufe.BasisSteuer + (steuerbaresEinkommen-stufe.Von)*stufe.Rate
			break
		}
	}
	if istVerheiratet {
		steuer *= 0.85
	}
	return math.Round(steuer)
}

// ── Vermögen ──────────────────────────────────────────────────────────────────

func berechneTotalVermoegen(v models.Vermoegen) float64 {
	total := 0.0
	for _, konto := range v.Bankguthaben {
		total += konto.Saldo
	}
	total += v.Wertschriften
	total += v.Fahrzeuge
	total += v.LebensversicherungRueckkauf
	total += v.UebrigesVermoegen
	return total
}

// ── Kirchensteuer ─────────────────────────────────────────────────────────────

func kirchensteuerfuss(konfession models.Konfession, p models.SteuerparameterDB) int {
	switch konfession {
	case models.KonfessionEvangelisch:
		if k, ok := p.Steuerfuesse.Kirche["evangelisch"]; ok {
			return k.Typisch
		}
	case models.KonfessionKatholisch:
		if k, ok := p.Steuerfuesse.Kirche["katholisch"]; ok {
			return k.Typisch
		}
	case models.KonfessionChristkatholisch:
		if k, ok := p.Steuerfuesse.Kirche["christkatholisch"]; ok {
			return k.Typisch
		}
	}
	return 0
}
