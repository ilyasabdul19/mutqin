import { createFileRoute } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { Button } from '../../components/ui/Button';

export const Route = createFileRoute('/_auth/login')({
  component: LoginPage,
});

function LoginPage() {
  const { t } = useTranslation();

  return (
    <div className="flex min-h-dvh flex-col items-center justify-center bg-gray-50 px-4">
      <div className="w-full max-w-sm rounded-lg border border-gray-200 bg-white p-6">
        <h1 className="mb-6 text-center text-2xl font-bold text-gray-900">
          {t('app.name')}
        </h1>
        <form className="flex flex-col gap-4" onSubmit={(e) => e.preventDefault()}>
          <label className="flex flex-col gap-1.5">
            <span className="text-sm font-medium text-gray-700">{t('auth.email')}</span>
            <input
              type="email"
              dir="ltr"
              className="rounded-lg border border-gray-200 px-3 py-3 text-base focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-600"
              placeholder={t('auth.emailPlaceholder')}
            />
          </label>
          <Button type="submit">{t('auth.sendCode')}</Button>
        </form>
      </div>
    </div>
  );
}
