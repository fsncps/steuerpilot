package models

// Extraction result structs — returned by Claude Vision, stored temporarily in session.

type Konfidenz struct {
	Gesamt         string   `json:"gesamt"`          // "hoch" | "mittel" | "tief"
	UnsichereFelder []string `json:"unsichere_felder"`
}

// Raw JSON shape returned by Claude for Lohnausweis
type LohnausweisRaw struct {
	ArbeitgeberName       string   `json:"arbeitgeber_name"`
	ArbeitgeberOrt        string   `json:"arbeitgeber_ort"`
	ArbeitnehmerName      string   `json:"arbeitnehmer_name"`
	AhvNummer             *string  `json:"ahv_nummer"`
	Bruttolohn            float64  `json:"ziff8_bruttolohn"`
	Sozialabgaben         float64  `json:"ziff9_sozialabgaben"`
	BvgOrdentlich         *float64 `json:"ziff10_1_bvg_ordentlich"`
	BvgEinkauf            *float64 `json:"ziff10_2_bvg_einkauf"`
	Nettolohn             float64  `json:"ziff11_nettolohn"`
	Quellensteuer         *float64 `json:"ziff12_quellensteuer"`
	SpesenEffektiv        *float64 `json:"ziff13_1_spesen_effektiv"`
	SpesenPauschal        *float64 `json:"ziff13_2_spesen_pauschal"`
	AussendienstProzent   *float64 `json:"ziff15_aussendienst_prozent"`
	Kinderzulagen         *float64 `json:"ziff5_kinderzulagen"`
	FeldFGAOderGeschaeftsauto bool `json:"feld_f_ga_oder_geschaeftsauto"`
	FeldGKantine          bool     `json:"feld_g_kantine"`
	Konfidenz             Konfidenz `json:"konfidenz"`
}

type KontoauszugRaw struct {
	Stichtag  string           `json:"stichtag"`
	Konten    []KontoRaw       `json:"konten"`
	Konfidenz Konfidenz        `json:"konfidenz"`
}

type KontoRaw struct {
	Bank              string   `json:"bank"`
	Kontonummer       *string  `json:"kontonummer"`
	IBAN              *string  `json:"iban"`
	Bezeichnung       string   `json:"bezeichnung"`
	Waehrung          string   `json:"waehrung"`
	Saldo             float64  `json:"saldo"`
	Zinsertrag        *float64 `json:"zinsertrag"`
	Verrechnungssteuer *float64 `json:"verrechnungssteuer"`
}

type Saeule3aRaw struct {
	Institut       string    `json:"institut"`
	Steuerjahr     int       `json:"steuerjahr"`
	Einzahlung     float64   `json:"einzahlung"`
	Art            string    `json:"art"`
	SaldoJahresende *float64 `json:"saldo_jahresende"`
	Konfidenz      Konfidenz `json:"konfidenz"`
}

// SessionExtractionResult holds the latest OCR result before the user accepts it.
type SessionExtractionResult struct {
	Type    string `json:"type"` // "lohnausweis" | "kontoauszug" | "3a"
	// Raw JSON bytes — one of the *Raw types above, stored as JSON for session serialization
	Payload []byte `json:"payload"`
}

type UploadedFile struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Extracted bool   `json:"extracted"`
}
