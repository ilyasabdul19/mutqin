import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';

const RTL_LANGUAGES = new Set(['ar']);

/**
 * Sets the document direction (rtl/ltr) and lang attribute
 * based on the current i18next language.
 */
export function useDirection() {
  const { i18n } = useTranslation();

  useEffect(() => {
    const applyDirection = (lng: string) => {
      const dir = RTL_LANGUAGES.has(lng) ? 'rtl' : 'ltr';
      document.documentElement.dir = dir;
      document.documentElement.lang = lng;
    };

    // Apply immediately for the current language
    applyDirection(i18n.language);

    // Listen for language changes
    i18n.on('languageChanged', applyDirection);
    return () => {
      i18n.off('languageChanged', applyDirection);
    };
  }, [i18n]);
}
