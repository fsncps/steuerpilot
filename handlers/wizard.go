package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"steuerpilot/internal/models"
	"steuerpilot/internal/session"
	"steuerpilot/internal/tax"
	"steuerpilot/templates/wizard"
)

var wizardSteps = []string{"personalien", "einkommen", "abzuege", "vermoegen", "zusammenfassung"}

func stepIndex(step string) int {
	for i, s := range wizardSteps {
		if s == step {
			return i
		}
	}
	return 0
}

func (h *Handler) WizardStep(c *fiber.Ctx) error {
	step := c.Params("step")
	sf := session.GetSteuerfall(c)
	switch step {
	case "personalien":
		gemeinden := tax.GetAlleGemeinden(h.params)
		return render(c, wizard.Personalien(sf, gemeinden))
	case "einkommen":
		return render(c, wizard.Einkommen(sf))
	case "abzuege":
		return render(c, wizard.Abzuege(sf))
	case "vermoegen":
		return render(c, wizard.Vermoegen(sf))
	case "zusammenfassung":
		ergebnis := tax.BerechneSteuern(sf, h.params)
		return render(c, wizard.Zusammenfassung(sf, ergebnis))
	default:
		return c.Redirect("/wizard/personalien")
	}
}

func (h *Handler) WizardSubmit(c *fiber.Ctx) error {
	step := c.Params("step")
	sf := session.GetSteuerfall(c)

	switch step {
	case "personalien":
		sf.Personalien.Vorname = c.FormValue("vorname")
		sf.Personalien.Nachname = c.FormValue("nachname")
		sf.Personalien.Geburtsdatum = c.FormValue("geburtsdatum")
		sf.Personalien.Zivilstand = models.Zivilstand(c.FormValue("zivilstand"))
		sf.Personalien.Konfession = models.Konfession(c.FormValue("konfession"))
		sf.Personalien.Gemeinde = c.FormValue("gemeinde")

	case "einkommen":
		sf.Einkommen.Haupterwerb.Arbeitgeber = c.FormValue("haupt_arbeitgeber")
		sf.Einkommen.Haupterwerb.Bruttolohn = parseFloat(c.FormValue("haupt_bruttolohn"))
		sf.Einkommen.Haupterwerb.Nettolohn = parseFloat(c.FormValue("haupt_nettolohn"))
		sf.Einkommen.Haupterwerb.AhvIvEoAlvNbuv = parseFloat(c.FormValue("haupt_ahv"))
		sf.Einkommen.Haupterwerb.BvgOrdentlich = parseFloat(c.FormValue("haupt_bvg"))
		sf.Einkommen.Haupterwerb.HatGA = c.FormValue("haupt_hatGA") == "1"
		sf.Einkommen.Haupterwerb.HatKantine = c.FormValue("haupt_hatKantine") == "1"
		sf.Einkommen.Haupterwerb.HatGeschaeftsauto = c.FormValue("haupt_hatGeschaeftsauto") == "1"
		sf.Einkommen.WertschriftenErtraege = parseFloat(c.FormValue("wertschriften_ertraege"))
		sf.Einkommen.Bankzinsen = parseFloat(c.FormValue("bankzinsen"))
		sf.Einkommen.Renten = parseFloat(c.FormValue("renten"))
		sf.Einkommen.UebrigeEinkuenfte = parseFloat(c.FormValue("uebrige_einkuenfte"))

	case "abzuege":
		sf.Abzuege.Berufskosten.Fahrkosten.Art = models.FahrkostenArt(c.FormValue("fahrkosten_art"))
		sf.Abzuege.Berufskosten.Fahrkosten.OevKosten = parseFloat(c.FormValue("fahrkosten_betrag"))
		sf.Abzuege.Berufskosten.Fahrkosten.Arbeitstage = parseInt(c.FormValue("fahrkosten_arbeitstage"))
		sf.Abzuege.Berufskosten.Verpflegung.Auswaertig = c.FormValue("verpflegung_auswaertig") == "1"
		sf.Abzuege.Berufskosten.Verpflegung.Kantine = c.FormValue("verpflegung_kantine") == "1"
		sf.Abzuege.Berufskosten.Verpflegung.Arbeitstage = parseInt(c.FormValue("verpflegung_arbeitstage"))
		sf.Abzuege.Saeule3a = parseFloat(c.FormValue("saeule3a"))
		sf.Abzuege.Versicherungspraemien = parseFloat(c.FormValue("versicherungspraemien"))
		sf.Abzuege.Krankheitskosten = parseFloat(c.FormValue("krankheitskosten"))
		sf.Abzuege.Schuldzinsen = parseFloat(c.FormValue("schuldzinsen"))
		sf.Abzuege.Unterhaltsbeitraege = parseFloat(c.FormValue("unterhaltsbeitraege"))
		sf.Abzuege.Spenden = parseFloat(c.FormValue("spenden"))
		sf.Abzuege.Weiterbildung = parseFloat(c.FormValue("weiterbildung"))

	case "vermoegen":
		// Use session count as authoritative — HTMX add/remove keeps it in sync
		count := len(sf.Vermoegen.Bankguthaben)
		konten := make([]models.Bankkonto, 0, count)
		for i := 0; i < count; i++ {
			konten = append(konten, models.Bankkonto{
				Bank:       c.FormValue("konto_bank_" + strconv.Itoa(i)),
				Saldo:      parseFloat(c.FormValue("konto_saldo_" + strconv.Itoa(i))),
				Zinsertrag: parseFloat(c.FormValue("konto_zins_" + strconv.Itoa(i))),
				Waehrung:   "CHF",
			})
		}
		sf.Vermoegen.Bankguthaben = konten
		sf.Vermoegen.Wertschriften = parseFloat(c.FormValue("wertschriften"))
		sf.Vermoegen.Fahrzeuge = parseFloat(c.FormValue("fahrzeuge"))
		sf.Vermoegen.LebensversicherungRueckkauf = parseFloat(c.FormValue("lebensversicherung_rueckkauf"))
		sf.Vermoegen.UebrigesVermoegen = parseFloat(c.FormValue("uebrigesVermoegen"))
		sf.Vermoegen.Schulden = parseFloat(c.FormValue("schulden"))
	}

	session.SaveSteuerfall(c, sf)

	idx := stepIndex(step)
	if idx+1 < len(wizardSteps) {
		return c.Redirect("/wizard/" + wizardSteps[idx+1])
	}
	return c.Redirect("/ergebnis")
}

func (h *Handler) WizardBack(c *fiber.Ctx) error {
	step := c.Params("step")
	idx := stepIndex(step)
	if idx > 0 {
		return c.Redirect("/wizard/" + wizardSteps[idx-1])
	}
	return c.Redirect("/upload")
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseInt(s string) int {
	if s == "" {
		return 0
	}
	i, _ := strconv.Atoi(s)
	return i
}
