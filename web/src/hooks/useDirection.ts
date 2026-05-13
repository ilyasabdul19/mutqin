import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

const RTL_LANGUAGES = new Set(['ar']);

export type Direction = 'rtl' | 'ltr';

function directionFor(lng: string): Direction {
  return RTL_LANGUAGES.has(lng) ? 'rtl' : 'ltr';
}

/**
 * Tracks the current direction (rtl/ltr) for the active i18next language
 * and keeps the document's `dir` and `lang` attributes in sync.
 *
 * Returns the current direction so callers can branch on it (e.g. for
 * direction-sensitive icons or layouts).
 */
export function useDirection(): Direction {
  const { i18n } = useTranslation();
  const [direction, setDirection] = useState<Direction>(() => directionFor(i18n.language));

  useEffect(() => {
    const applyDirection = (lng: string) => {
      const dir = directionFor(lng);
      document.documentElement.setAttribute('dir', dir);
      document.documentElement.setAttribute('lang', lng);
      setDirection(dir);
    };

    // Apply immediately for the current language
    applyDirection(i18n.language);

    // Listen for language changes
    i18n.on('languageChanged', applyDirection);
    return () => {
      i18n.off('languageChanged', applyDirection);
    };
  }, [i18n]);

  return direction;
}
