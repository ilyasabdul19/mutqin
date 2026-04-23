import { useTranslation } from 'react-i18next';
import { useOfflineStatus } from '../../hooks/useOfflineStatus';

interface TopBarProps {
  title: string;
}

export function TopBar({ title }: TopBarProps) {
  const { t, i18n } = useTranslation();
  const { isOnline } = useOfflineStatus();

  const toggleLanguage = () => {
    const next = i18n.language === 'ar' ? 'so' : 'ar';
    i18n.changeLanguage(next);
  };

  return (
    <header className="flex h-12 items-center justify-between border-b border-gray-200 bg-white px-4">
      <h1 className="text-start text-lg font-semibold text-gray-900">{t(title)}</h1>
      <div className="flex items-center gap-3">
        <span
          role="status"
          className={`inline-block h-2 w-2 rounded-full ${isOnline ? 'bg-green-500' : 'bg-amber-500'}`}
          aria-label={isOnline ? t('common.online') : t('common.offline')}
        />
        <button
          onClick={toggleLanguage}
          className="rounded-md px-2 py-1 text-sm font-medium text-gray-600 hover:bg-gray-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-600"
          aria-label={t('common.switchLanguage')}
        >
          {i18n.language === 'ar' ? 'SO' : 'عر'}
        </button>
      </div>
    </header>
  );
}
