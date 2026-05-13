import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';

// Eastern Arabic-Indic digits 0-9.
const EASTERN_ARABIC_DIGITS = ['٠', '١', '٢', '٣', '٤', '٥', '٦', '٧', '٨', '٩'] as const;

const WESTERN_DIGIT_RE = /[0-9]/g;

function toEasternArabic(input: string): string {
  return input.replace(WESTERN_DIGIT_RE, (d) => EASTERN_ARABIC_DIGITS[Number(d)]);
}

interface UseNumeralsResult {
  /**
   * Converts any Western Arabic digits (0-9) in the input to Eastern Arabic
   * numerals (٠-٩) when the active language is Arabic. For other languages
   * the value is returned as a string unchanged.
   */
  toLocaleDigits: (value: number | string) => string;
}

/**
 * Hook that exposes a locale-aware digit converter. The returned function
 * is stable for a given language so it is safe to use in component bodies
 * and effect deps.
 */
export function useNumerals(): UseNumeralsResult {
  const { i18n } = useTranslation();
  const isArabic = i18n.language === 'ar' || i18n.language.startsWith('ar-');

  const toLocaleDigits = useCallback(
    (value: number | string): string => {
      const str = typeof value === 'number' ? String(value) : value;
      return isArabic ? toEasternArabic(str) : str;
    },
    [isArabic],
  );

  return { toLocaleDigits };
}
