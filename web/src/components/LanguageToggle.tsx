import { useTranslation } from 'react-i18next';

type SupportedLng = 'ar' | 'so';

const NEXT_LNG: Record<SupportedLng, SupportedLng> = {
  ar: 'so',
  so: 'ar',
};

// What we render on the button: the label of the language you will switch TO.
const BUTTON_LABEL: Record<SupportedLng, string> = {
  ar: 'SO',
  so: 'عر',
};

interface LanguageToggleProps {
  className?: string;
}

/**
 * Small button that toggles the i18next language between Arabic and Somali.
 * Persistence and document direction are handled by i18n.ts + useDirection.
 */
export function LanguageToggle({ className = '' }: LanguageToggleProps) {
  const { t, i18n } = useTranslation();

  const current: SupportedLng = i18n.language === 'so' ? 'so' : 'ar';
  const next = NEXT_LNG[current];

  const onClick = () => {
    void i18n.changeLanguage(next);
  };

  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={t('common.switchLanguage')}
      className={
        'inline-flex min-h-8 min-w-10 items-center justify-center rounded-md px-2 py-1 text-sm font-medium text-gray-600 hover:bg-gray-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-600 ' +
        className
      }
    >
      {BUTTON_LABEL[current]}
    </button>
  );
}
