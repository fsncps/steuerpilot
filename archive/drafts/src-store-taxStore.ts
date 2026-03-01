// src/store/taxStore.ts
// Globaler State für den Steuerfall

import { create } from 'zustand';
import type {
  Steuerfall,
  Personalien,
  Einkommen,
  Abzuege,
  Vermoegen,
  Steuerergebnis,
  Optimierung,
  Erwerbseinkommen,
  Berufskosten,
  Kind,
} from '@/lib/tax/types';

// Wizard-Schritte
export type WizardStep = 'upload' | 'personalien' | 'einkommen' | 'abzuege' | 'vermoegen' | 'zusammenfassung';

const WIZARD_STEPS: WizardStep[] = ['upload', 'personalien', 'einkommen', 'abzuege', 'vermoegen', 'zusammenfassung'];

// Default-Werte
const defaultErwerbseinkommen: Erwerbseinkommen = {
  arbeitgeber: '',
  bruttolohn: 0,
  nettolohn: 0,
  ahvIvEoAlvNbuv: 0,
  bvgOrdentlich: 0,
  bvgEinkauf: 0,
  quellensteuer: 0,
  spesenEffektiv: 0,
  spesenPauschal: 0,
  aussendienstProzent: 0,
  hatGeschaeftsauto: false,
  hatGA: false,
  hatKantine: false,
};

const defaultBerufskosten: Berufskosten = {
  fahrkosten: {
    art: 'oev',
    distanzKm: 0,
    arbeitstage: 220,
  },
  verpflegung: {
    auswärtig: false,
    kantine: false,
    arbeitstage: 220,
  },
  uebrigeBerufskosten: 0,
  weiterbildungskosten: 0,
};

const defaultPersonalien: Personalien = {
  vorname: '',
  nachname: '',
  geburtsdatum: '',
  zivilstand: 'alleinstehend',
  konfession: 'keine',
  gemeinde: 'Gommiswald',
  kinder: [],
};

const defaultEinkommen: Einkommen = {
  haupterwerb: { ...defaultErwerbseinkommen },
  nebenerwerb: [],
  wertschriftenErtraege: 0,
  bankzinsen: 0,
  beteiligungsErtraege: 0,
  liegenschaftenEinkuenfte: 0,
  uebrigeEinkuenfte: 0,
  renten: 0,
  kinderzulagen: 0,
};

const defaultAbzuege: Abzuege = {
  berufskosten: { ...defaultBerufskosten },
  sozialabgaben: 0,
  bvgBeitraege: 0,
  saeule3a: 0,
  versicherungspraemien: 0,
  krankheitskosten: 0,
  schuldzinsen: 0,
  unterhaltsbeitraege: 0,
  spenden: 0,
  weiterbildung: 0,
  liegenschaftsunterhalt: 0,
};

const defaultVermoegen: Vermoegen = {
  bankguthaben: [],
  wertschriften: 0,
  fahrzeuge: 0,
  lebensversicherungRueckkauf: 0,
  uebrigesVermoegen: 0,
  schulden: 0,
};

// Store Interface
interface TaxStore {
  // State
  steuerperiode: number;
  currentStep: WizardStep;
  personalien: Personalien;
  einkommen: Einkommen;
  abzuege: Abzuege;
  vermoegen: Vermoegen;
  ergebnis: Steuerergebnis | null;
  optimierungen: Optimierung[];
  isLoading: boolean;
  uploadedFiles: { name: string; type: string; extracted: boolean }[];

  // Navigation
  setStep: (step: WizardStep) => void;
  nextStep: () => void;
  prevStep: () => void;
  canGoNext: () => boolean;

  // Daten setzen
  setPersonalien: (data: Partial<Personalien>) => void;
  setEinkommen: (data: Partial<Einkommen>) => void;
  setHaupterwerb: (data: Partial<Erwerbseinkommen>) => void;
  setAbzuege: (data: Partial<Abzuege>) => void;
  setBerufskosten: (data: Partial<Berufskosten>) => void;
  setVermoegen: (data: Partial<Vermoegen>) => void;
  setErgebnis: (ergebnis: Steuerergebnis) => void;
  setOptimierungen: (optimierungen: Optimierung[]) => void;

  // Kinder
  addKind: (kind: Kind) => void;
  removeKind: (index: number) => void;

  // Upload
  addUploadedFile: (file: { name: string; type: string }) => void;
  markFileExtracted: (name: string) => void;

  // Hilfsfunktionen
  getSteuerfall: () => Steuerfall;
  reset: () => void;
  setLoading: (loading: boolean) => void;
}

export const useTaxStore = create<TaxStore>((set, get) => ({
  // Initial State
  steuerperiode: 2024,
  currentStep: 'upload',
  personalien: { ...defaultPersonalien },
  einkommen: { ...defaultEinkommen },
  abzuege: { ...defaultAbzuege },
  vermoegen: { ...defaultVermoegen },
  ergebnis: null,
  optimierungen: [],
  isLoading: false,
  uploadedFiles: [],

  // Navigation
  setStep: (step) => set({ currentStep: step }),
  
  nextStep: () => {
    const { currentStep } = get();
    const currentIndex = WIZARD_STEPS.indexOf(currentStep);
    if (currentIndex < WIZARD_STEPS.length - 1) {
      set({ currentStep: WIZARD_STEPS[currentIndex + 1] });
    }
  },

  prevStep: () => {
    const { currentStep } = get();
    const currentIndex = WIZARD_STEPS.indexOf(currentStep);
    if (currentIndex > 0) {
      set({ currentStep: WIZARD_STEPS[currentIndex - 1] });
    }
  },

  canGoNext: () => {
    const { currentStep, personalien } = get();
    switch (currentStep) {
      case 'upload':
        return true; // Upload ist optional
      case 'personalien':
        return personalien.vorname.length > 0 && personalien.nachname.length > 0 && personalien.gemeinde.length > 0;
      case 'einkommen':
        return true;
      case 'abzuege':
        return true;
      case 'vermoegen':
        return true;
      case 'zusammenfassung':
        return false; // Letzter Schritt
      default:
        return false;
    }
  },

  // Daten setzen
  setPersonalien: (data) =>
    set((state) => ({ personalien: { ...state.personalien, ...data } })),

  setEinkommen: (data) =>
    set((state) => ({ einkommen: { ...state.einkommen, ...data } })),

  setHaupterwerb: (data) =>
    set((state) => ({
      einkommen: {
        ...state.einkommen,
        haupterwerb: { ...state.einkommen.haupterwerb, ...data },
      },
    })),

  setAbzuege: (data) =>
    set((state) => ({ abzuege: { ...state.abzuege, ...data } })),

  setBerufskosten: (data) =>
    set((state) => ({
      abzuege: {
        ...state.abzuege,
        berufskosten: { ...state.abzuege.berufskosten, ...data },
      },
    })),

  setVermoegen: (data) =>
    set((state) => ({ vermoegen: { ...state.vermoegen, ...data } })),

  setErgebnis: (ergebnis) => set({ ergebnis }),

  setOptimierungen: (optimierungen) => set({ optimierungen }),

  // Kinder
  addKind: (kind) =>
    set((state) => ({
      personalien: {
        ...state.personalien,
        kinder: [...state.personalien.kinder, kind],
      },
    })),

  removeKind: (index) =>
    set((state) => ({
      personalien: {
        ...state.personalien,
        kinder: state.personalien.kinder.filter((_, i) => i !== index),
      },
    })),

  // Upload
  addUploadedFile: (file) =>
    set((state) => ({
      uploadedFiles: [...state.uploadedFiles, { ...file, extracted: false }],
    })),

  markFileExtracted: (name) =>
    set((state) => ({
      uploadedFiles: state.uploadedFiles.map((f) =>
        f.name === name ? { ...f, extracted: true } : f
      ),
    })),

  // Hilfsfunktionen
  getSteuerfall: () => {
    const state = get();
    return {
      steuerperiode: state.steuerperiode,
      personalien: state.personalien,
      einkommen: state.einkommen,
      abzuege: state.abzuege,
      vermoegen: state.vermoegen,
      ergebnis: state.ergebnis ?? undefined,
      optimierungen: state.optimierungen.length > 0 ? state.optimierungen : undefined,
    };
  },

  reset: () =>
    set({
      currentStep: 'upload',
      personalien: { ...defaultPersonalien },
      einkommen: { ...defaultEinkommen },
      abzuege: { ...defaultAbzuege },
      vermoegen: { ...defaultVermoegen },
      ergebnis: null,
      optimierungen: [],
      uploadedFiles: [],
    }),

  setLoading: (loading) => set({ isLoading: loading }),
}));
