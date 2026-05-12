import i18n from 'i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import { initReactI18next } from 'react-i18next';

import ar from './locales/ar.json';
import so from './locales/so.json';

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      ar: { translation: ar },
      so: { translation: so },
    },
    fallbackLng: 'ar',
    supportedLngs: ['ar', 'so'],
    nonExplicitSupportedLngs: true,
    detection: {
      // Order: previously-chosen language (persisted) > <html lang> > navigator > fallback.
      order: ['localStorage', 'htmlTag', 'navigator'],
      caches: ['localStorage'],
      lookupLocalStorage: 'i18nextLng',
    },
    interpolation: {
      escapeValue: false, // React already escapes output
    },
  });

export default i18n;
