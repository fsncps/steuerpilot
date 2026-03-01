package tax

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"steuerpilot/internal/models"
)

func basisSteuerfall() models.Steuerfall {
	sf := models.NewDefaultSteuerfall()
	sf.Personalien = models.Personalien{
		Vorname:      "Hans",
		Nachname:     "Muster",
		Geburtsdatum: "1980-01-01",
		Zivilstand:   models.ZivilstandAlleinstehend,
		Konfession:   models.KonfessionEvangelisch,
		Gemeinde:     "St. Gallen",
		Kinder:       []models.Kind{},
	}
	sf.Einkommen.Haupterwerb = models.Erwerbseinkommen{
		Arbeitgeber:   "Muster AG",
		Bruttolohn:    80000,
		Nettolohn:     73000,
		AhvIvEoAlvNbuv: 5800,
		BvgOrdentlich: 4000,
	}
	sf.Einkommen.WertschriftenErtraege = 500
	sf.Einkommen.Bankzinsen = 100
	sf.Abzuege.Sozialabgaben = 5800
	sf.Abzuege.BvgBeitraege = 4000
	sf.Abzuege.Saeule3a = 7056
	sf.Abzuege.Versicherungspraemien = 3400
	sf.Abzuege.Berufskosten.Fahrkosten = models.Fahrkosten{
		Art: models.FahrkostenOev, Arbeitstage: 220, OevKosten: 1800,
	}
	sf.Abzuege.Berufskosten.Verpflegung = models.Verpflegung{Auswaertig: true, Arbeitstage: 220}
	sf.Vermoegen.Bankguthaben = []models.Bankkonto{
		{Bank: "UBS", Bezeichnung: "Konto", Saldo: 20000, Waehrung: "CHF", Zinsertrag: 100},
	}
	return sf
}

func TestBerechneSteuern_Grundfall(t *testing.T) {
	params, err := LoadSteuerparameter("../../docs/steuerparameter.json")
	require.NoError(t, err)

	sf := basisSteuerfall()
	ergebnis := BerechneSteuern(sf, params)

	assert.Equal(t, 80600.0, ergebnis.TotalEinkommen, "Gesamteinkommen")
	assert.Greater(t, ergebnis.Kantonssteuer, 0.0)
	assert.Greater(t, ergebnis.Gemeindesteuer, 0.0)
	assert.Greater(t, ergebnis.Bundessteuer, 0.0)

	// St. Gallen (138%) / Kanton (105%) ratio
	ratio := ergebnis.Gemeindesteuer / ergebnis.Kantonssteuer
	assert.InDelta(t, 138.0/105.0, ratio, 0.05)
}

func TestBerechneSteuern_TiefereGemeindesteuerRapperswilJona(t *testing.T) {
	params, _ := LoadSteuerparameter("../../docs/steuerparameter.json")
	sg := basisSteuerfall()
	rj := basisSteuerfall()
	rj.Personalien.Gemeinde = "Rapperswil-Jona"

	sgE := BerechneSteuern(sg, params)
	rjE := BerechneSteuern(rj, params)
	assert.Less(t, rjE.Gemeindesteuer, sgE.Gemeindesteuer)
}

func TestBerechneSteuern_Splitting(t *testing.T) {
	params, _ := LoadSteuerparameter("../../docs/steuerparameter.json")
	single := basisSteuerfall()
	single.Einkommen.Haupterwerb.Bruttolohn = 120000

	married := basisSteuerfall()
	married.Personalien.Zivilstand = models.ZivilstandVerheiratet
	married.Einkommen.Haupterwerb.Bruttolohn = 120000

	singleE := BerechneSteuern(single, params)
	marriedE := BerechneSteuern(married, params)
	assert.Less(t, marriedE.Kantonssteuer, singleE.Kantonssteuer, "Splitting muss Steuer senken")
}
