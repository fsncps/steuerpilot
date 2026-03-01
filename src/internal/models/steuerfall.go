package models

// ── Enumerations ──────────────────────────────────────────────────────────────

type Zivilstand string

const (
	ZivilstandAlleinstehend           Zivilstand = "alleinstehend"
	ZivilstandVerheiratet             Zivilstand = "verheiratet"
	ZivilstandGeschieden              Zivilstand = "geschieden"
	ZivilstandVerwitwet               Zivilstand = "verwitwet"
	ZivilstandGetrennt                Zivilstand = "getrennt"
	ZivilstandEingetragenePartnerschaft Zivilstand = "eingetragene_partnerschaft"
)

type Konfession string

const (
	KonfessionEvangelisch     Konfession = "evangelisch"
	KonfessionKatholisch      Konfession = "katholisch"
	KonfessionChristkatholisch Konfession = "christkatholisch"
	KonfessionAndere          Konfession = "andere"
	KonfessionKeine           Konfession = "keine"
)

type FahrkostenArt string

const (
	FahrkostenOev       FahrkostenArt = "oev"
	FahrkostenAuto      FahrkostenArt = "auto"
	FahrkostenMotorrad  FahrkostenArt = "motorrad"
	FahrkostenVelo      FahrkostenArt = "velo"
	FahrkostenKeine     FahrkostenArt = "keine"
)

// ── Top-level ─────────────────────────────────────────────────────────────────

type Steuerfall struct {
	Steuerperiode int             `json:"steuerperiode"`
	Personalien   Personalien     `json:"personalien"`
	Einkommen     Einkommen       `json:"einkommen"`
	Abzuege       Abzuege         `json:"abzuege"`
	Vermoegen     Vermoegen       `json:"vermoegen"`
	Ergebnis      *Steuerergebnis `json:"ergebnis,omitempty"`
	Optimierungen []Optimierung   `json:"optimierungen,omitempty"`
}

type Personalien struct {
	Vorname     string     `json:"vorname"`
	Nachname    string     `json:"nachname"`
	Geburtsdatum string    `json:"geburtsdatum"`
	Zivilstand  Zivilstand `json:"zivilstand"`
	Konfession  Konfession `json:"konfession"`
	Gemeinde    string     `json:"gemeinde"`
	Kinder      []Kind     `json:"kinder"`
	Partner     *Personalien `json:"partner,omitempty"`
}

type Kind struct {
	Vorname         string  `json:"vorname"`
	Geburtsdatum    string  `json:"geburtsdatum"`
	InAusbildung    bool    `json:"inAusbildung"`
	Fremdbetreuung  bool    `json:"fremdbetreuung"`
	Betreuungskosten float64 `json:"betreuungskosten,omitempty"`
}

type Einkommen struct {
	Haupterwerb          Erwerbseinkommen   `json:"haupterwerb"`
	Nebenerwerb          []Erwerbseinkommen `json:"nebenerwerb"`
	WertschriftenErtraege float64           `json:"wertschriftenErtraege"`
	Bankzinsen           float64            `json:"bankzinsen"`
	BeteiligungsErtraege float64            `json:"beteiligungsErtraege"`
	LiegenschaftenEinkuenfte float64        `json:"liegenschaftenEinkuenfte"`
	UebrigeEinkuenfte    float64            `json:"uebrigeEinkuenfte"`
	Renten               float64            `json:"renten"`
	Kinderzulagen        float64            `json:"kinderzulagen"`
}

type Erwerbseinkommen struct {
	Arbeitgeber        string        `json:"arbeitgeber"`
	Bruttolohn         float64       `json:"bruttolohn"`
	Nettolohn          float64       `json:"nettolohn"`
	AhvIvEoAlvNbuv     float64       `json:"ahvIvEoAlvNbuv"`
	BvgOrdentlich      float64       `json:"bvgOrdentlich"`
	BvgEinkauf         float64       `json:"bvgEinkauf"`
	Quellensteuer      float64       `json:"quellensteuer"`
	SpesenEffektiv     float64       `json:"spesenEffektiv"`
	SpesenPauschal     float64       `json:"spesenPauschal"`
	AussendienstProzent float64      `json:"aussendienstProzent"`
	HatGeschaeftsauto  bool          `json:"hatGeschaeftsauto"`
	HatGA              bool          `json:"hatGA"`
	HatKantine         bool          `json:"hatKantine"`
}

type Abzuege struct {
	Berufskosten         Berufskosten `json:"berufskosten"`
	Sozialabgaben        float64      `json:"sozialabgaben"`
	BvgBeitraege         float64      `json:"bvgBeitraege"`
	Saeule3a             float64      `json:"saeule3a"`
	Versicherungspraemien float64     `json:"versicherungspraemien"`
	Krankheitskosten     float64      `json:"krankheitskosten"`
	Schuldzinsen         float64      `json:"schuldzinsen"`
	Unterhaltsbeitraege  float64      `json:"unterhaltsbeitraege"`
	Spenden              float64      `json:"spenden"`
	Weiterbildung        float64      `json:"weiterbildung"`
	Liegenschaftsunterhalt float64    `json:"liegenschaftsunterhalt"`
}

type Berufskosten struct {
	Fahrkosten          Fahrkosten  `json:"fahrkosten"`
	Verpflegung         Verpflegung `json:"verpflegung"`
	UebrigeBerufskosten float64     `json:"uebrigeBerufskosten"`
	Weiterbildungskosten float64    `json:"weiterbildungskosten"`
}

type Fahrkosten struct {
	Art        FahrkostenArt `json:"art"`
	DistanzKm  float64       `json:"distanzKm"`
	Arbeitstage int          `json:"arbeitstage"`
	OevKosten  float64       `json:"oevKosten"`
}

type Verpflegung struct {
	Auswaertig  bool `json:"auswaertig"`
	Kantine     bool `json:"kantine"`
	Arbeitstage int  `json:"arbeitstage"`
}

type Vermoegen struct {
	Bankguthaben              []Bankkonto `json:"bankguthaben"`
	Wertschriften             float64     `json:"wertschriften"`
	Fahrzeuge                 float64     `json:"fahrzeuge"`
	LebensversicherungRueckkauf float64   `json:"lebensversicherungRueckkauf"`
	UebrigesVermoegen         float64     `json:"uebrigesVermoegen"`
	Schulden                  float64     `json:"schulden"`
}

type Bankkonto struct {
	Bank              string  `json:"bank"`
	Bezeichnung       string  `json:"bezeichnung"`
	IBAN              string  `json:"iban,omitempty"`
	Saldo             float64 `json:"saldo"`
	Waehrung          string  `json:"waehrung"`
	Zinsertrag        float64 `json:"zinsertrag"`
	Verrechnungssteuer float64 `json:"verrechnungssteuer"`
}

type Steuerergebnis struct {
	TotalEinkommen          float64 `json:"totalEinkommen"`
	TotalAbzuege            float64 `json:"totalAbzuege"`
	SteuerbaresEinkommen    float64 `json:"steuerbaresEinkommen"`
	SteuerbaresEinkommenBund float64 `json:"steuerbaresEinkommenBund"`
	TotalVermoegen          float64 `json:"totalVermoegen"`
	TotalSchulden           float64 `json:"totalSchulden"`
	SteuerbaresVermoegen    float64 `json:"steuerbaresVermoegen"`
	EinfacheSteuer          float64 `json:"einfacheSteuer"`
	Kantonssteuer           float64 `json:"kantonssteuer"`
	Gemeindesteuer          float64 `json:"gemeindesteuer"`
	Kirchensteuer           float64 `json:"kirchensteuer"`
	Bundessteuer            float64 `json:"bundessteuer"`
	VermoegensSteuerKanton  float64 `json:"vermoegensSteuerKanton"`
	VermoegensSteuerGemeinde float64 `json:"vermoegensSteuerGemeinde"`
	TotalSteuer             float64 `json:"totalSteuer"`
	Gemeinde                string  `json:"gemeinde"`
	SteuerfussGemeinde      int     `json:"steuerfussGemeinde"`
	SteuerfussKanton        int     `json:"steuerfussKanton"`
	SteuerfussKirche        int     `json:"steuerfussKirche"`
	Steuerperiode           int     `json:"steuerperiode"`
}

type Optimierung struct {
	Titel               string   `json:"titel"`
	Beschreibung        string   `json:"beschreibung"`
	SparpotenzialMin    *float64 `json:"sparpotenzial_min"`
	SparpotenzialMax    *float64 `json:"sparpotenzial_max"`
	Aufwand             string   `json:"aufwand"`
	Zeitrahmen          string   `json:"zeitrahmen"`
	Kategorie           string   `json:"kategorie"`
	GesetzlicheGrundlage string  `json:"gesetzliche_grundlage"`
}

// NewDefaultSteuerfall returns a zero-value Steuerfall with sensible defaults.
func NewDefaultSteuerfall() Steuerfall {
	return Steuerfall{
		Steuerperiode: 2024,
		Abzuege: Abzuege{
			Berufskosten: Berufskosten{
				Fahrkosten:  Fahrkosten{Art: FahrkostenOev, Arbeitstage: 220},
				Verpflegung: Verpflegung{Arbeitstage: 220},
			},
		},
	}
}
