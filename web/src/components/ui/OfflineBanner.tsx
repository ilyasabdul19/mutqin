import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useOfflineStatus } from '../../hooks/useOfflineStatus';

type BannerState = 'hidden' | 'offline' | 'syncing' | 'synced' | 'error';

export function OfflineBanner() {
  const { t } = useTranslation();
  const { isOnline, isSyncing } = useOfflineStatus();
  const [state, setState] = useState<BannerState>('hidden');
  const [wasOffline, setWasOffline] = useState(false);

  useEffect(() => {
    if (!isOnline) {
      setState('offline');
      setWasOffline(true);
    } else if (isSyncing) {
      setState('syncing');
    } else if (wasOffline) {
      setState('synced');
      const timer = setTimeout(() => {
        setState('hidden');
        setWasOffline(false);
      }, 2000);
      return () => clearTimeout(timer);
    } else {
      setState('hidden');
    }
  }, [isOnline, isSyncing, wasOffline]);

  if (state === 'hidden') return null;

  const config: Record<Exclude<BannerState, 'hidden'>, { dot: string; text: string }> = {
    offline: { dot: 'bg-amber-500', text: t('common.offline') },
    syncing: { dot: 'bg-green-500 animate-pulse', text: t('common.syncing') },
    synced: { dot: 'bg-green-500', text: t('common.saved') },
    error: { dot: 'bg-red-500', text: t('common.retry') },
  };

  const { dot, text } = config[state];

  return (
    <div
      role="status"
      aria-live="polite"
      className="flex h-8 items-center justify-center gap-2 bg-gray-100 text-sm text-gray-700"
    >
      <span className={`inline-block h-2 w-2 rounded-full ${dot}`} aria-hidden="true" />
      {text}
    </div>
  );
}
