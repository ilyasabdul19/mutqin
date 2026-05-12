import { createFileRoute } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { LanguageToggle } from '../../components/LanguageToggle';
import { useNumerals } from '../../hooks/useNumerals';

export const Route = createFileRoute('/_app/')({
  component: HomePage,
});

function HomePage() {
  const { t } = useTranslation();
  const { toLocaleDigits } = useNumerals();

  return (
    <section className="flex flex-1 flex-col items-center justify-center gap-6 p-8 text-center">
      <h1 className="text-3xl font-semibold text-gray-900">{t('app.name')}</h1>
      <p className="text-base text-gray-500">{t('app.tagline')}</p>
      <p className="text-sm text-gray-600">
        <span className="font-mono">{toLocaleDigits(2026)}</span>
      </p>
      <LanguageToggle className="border border-gray-200" />
    </section>
  );
}
