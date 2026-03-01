package models

import "encoding/json"

// SteuerparameterDB is loaded once at startup from docs/steuerparameter.json.
type SteuerparameterDB struct {
	Steuerperiode int           `json:"steuerperiode"`
	Kanton        string        `json:"kanton"`
	Tarif         Tarif         `json:"tarif"`
	Steuerfuesse  Steuerfuesse  `json:"steuerfuesse"`
	Abzuege       AbzugsConfig  `json:"abzuege"`
	Bundessteuer  BundessteuerConfig `json:"bundessteuer"`
}

type Tarif struct {
	Einkommenssteuer EinkommenssteuerTarif `json:"einkommenssteuer"`
	Vermoegenssteuer VermoegenssteuerTarif `json:"vermoegenssteuer"`
}

type EinkommenssteuerTarif struct {
	Stufen                   []TarifStufe `json:"stufen"`
	MaxRate                  float64      `json:"maxRate"`
	MaxEinkommenGemeinsam    float64      `json:"maxEinkommenGemeinsam"`
	MaxEinkommenAlleinstehend float64     `json:"maxEinkommenAlleinstehend"`
}

type TarifStufe struct {
	Von        float64 `json:"von"`
	Bis        float64 `json:"bis"`
	BasisSteuer float64 `json:"basisSteuer"`
	Rate       float64 `json:"rate"`
}

type VermoegenssteuerTarif struct {
	Rate             float64 `json:"rate"`
	FreibetragPerson float64 `json:"freibetragPerson"`
	FreibetragKind   float64 `json:"freibetragKind"`
}

type Steuerfuesse struct {
	Kanton   int            `json:"kanton"`
	Gemeinden map[string]int `json:"gemeinden"`
	Kirche   map[string]KircheConfig `json:"kirche"`
}

type KircheConfig struct {
	Typisch int `json:"typisch"`
	Min     int `json:"min"`
	Max     int `json:"max"`
}

// UnmarshalJSON handles both plain-number and object forms in steuerparameter.json.
// e.g. "christkatholisch": 24  vs  "evangelisch": {"min":20,"max":28,"typisch":24}
func (k *KircheConfig) UnmarshalJSON(data []byte) error {
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		k.Typisch, k.Min, k.Max = n, n, n
		return nil
	}
	type alias KircheConfig
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*k = KircheConfig(a)
	return nil
}

type AbzugsConfig struct {
	Berufskosten   BerufskostenConfig   `json:"berufskosten"`
	Vorsorge       VorsorgeConfig       `json:"vorsorge"`
	Versicherungen VersicherungenConfig `json:"versicherungen"`
	Krankheitskosten KrankheitskostenConfig `json:"krankheitskosten"`
	WeiterbildungMax float64            `json:"weiterbildungMax"`
	SchuldzinsenMaxBasis float64        `json:"schuldzinsenMaxBasis"`
	SpendenMaxProzent float64           `json:"spendenMaxProzent"`
	Sozialabzuege    SozialabzuegeConfig `json:"sozialabzuege"`
}

type BerufskostenConfig struct {
	FahrkostenMax      float64 `json:"fahrkostenMax"`
	FahrkostenMaxBund  float64 `json:"fahrkostenMaxBund"`
	KmAuto             float64 `json:"kmAuto"`
	KmMotorrad         float64 `json:"kmMotorrad"`
	VeloPauschale      float64 `json:"veloPauschale"`
	VerpflegungTag     float64 `json:"verpflegungTag"`
	VerpflegungMax     float64 `json:"verpflegungMax"`
	VerpflegungKantineTag float64 `json:"verpflegungKantineTag"`
	VerpflegungKantineMax float64 `json:"verpflegungKantineMax"`
	UebrigeMin         float64 `json:"uebrigeMin"`
	UebrigeMax         float64 `json:"uebrigeMax"`
	UebrigeProzent     float64 `json:"uebrigeProzent"`
}

type VorsorgeConfig struct {
	Saeule3aMitPk        float64 `json:"saeule3aMitPk"`
	Saeule3aOhnePk       float64 `json:"saeule3aOhnePk"`
	Saeule3aOhnePkProzent float64 `json:"saeule3aOhnePkProzent"`
}

type VersicherungenConfig struct {
	Alleinstehend           float64 `json:"alleinstehend"`
	AlleinstehendOhneVorsorge float64 `json:"alleinstehendOhneVorsorge"`
	Gemeinsam               float64 `json:"gemeinsam"`
	GemeinsamOhneVorsorge   float64 `json:"gemeinsamOhneVorsorge"`
	ProKind                 float64 `json:"proKind"`
}

type KrankheitskostenConfig struct {
	SelbstbehaltProzent float64 `json:"selbstbehaltProzent"`
}

type SozialabzuegeConfig struct {
	KinderabzugVorschule  float64 `json:"kinderabzugVorschule"`
	KinderabzugAusbildung float64 `json:"kinderabzugAusbildung"`
}

type BundessteuerConfig struct {
	StufenAlleinstehend     []TarifStufe `json:"stufen_alleinstehend"`
	FahrkostenMax           float64      `json:"fahrkostenMax"`
	VersicherungenAlleinstehend float64  `json:"versicherungenAlleinstehend"`
	VersicherungenGemeinsam float64      `json:"versicherungenGemeinsam"`
	VersicherungenProKind   float64      `json:"versicherungenProKind"`
	Kinderabzug             float64      `json:"kinderabzug"`
	WeiterbildungMax        float64      `json:"weiterbildungMax"`
}
