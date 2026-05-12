import { useTranslation } from 'react-i18next';
import { useOfflineStatus } from '../../hooks/useOfflineStatus';
import { LanguageToggle } from '../LanguageToggle';

interface TopBarProps {
  title: string;
}

export function TopBar({ title }: TopBarProps) {
  const { t } = useTranslation();
  const { isOnline } = useOfflineStatus();

  return (
    <header className="flex h-12 items-center justify-between border-b border-gray-200 bg-white px-4">
      <h1 className="text-start text-lg font-semibold text-gray-900">{t(title)}</h1>
      <div className="flex items-center gap-3">
        <span
          role="status"
          className={`inline-block h-2 w-2 rounded-full ${isOnline ? 'bg-green-500' : 'bg-amber-500'}`}
          aria-label={isOnline ? t('common.online') : t('common.offline')}
        />
        <LanguageToggle />
      </div>
    </header>
  );
}
