import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

import ar from './locales/ar.json';
import so from './locales/so.json';

const savedLng = (() => {
  try {
    return localStorage.getItem('i18nextLng');
  } catch {
    return null;
  }
})();

i18n.use(initReactI18next).init({
  resources: {
    ar: { translation: ar },
    so: { translation: so },
  },
  lng: savedLng || 'ar',
  fallbackLng: 'ar',
  interpolation: {
    escapeValue: false, // React already escapes output
  },
});

i18n.on('languageChanged', (lng) => {
  try {
    localStorage.setItem('i18nextLng', lng);
  } catch {}
});

export default i18n;
