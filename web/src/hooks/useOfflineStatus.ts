import { useEffect, useState } from 'react';

interface OfflineStatus {
  isOnline: boolean;
  isSyncing: boolean;
  syncQueueCount: number;
}

/**
 * Tracks browser online/offline state.
 * isSyncing and syncQueueCount are placeholders for future sync-queue integration.
 */
export function useOfflineStatus(): OfflineStatus {
  const [isOnline, setIsOnline] = useState(
    typeof navigator !== 'undefined' ? navigator.onLine : true,
  );

  useEffect(() => {
    const handleOnline = () => setIsOnline(true);
    const handleOffline = () => setIsOnline(false);

    window.addEventListener('online', handleOnline);
    window.addEventListener('offline', handleOffline);

    return () => {
      window.removeEventListener('online', handleOnline);
      window.removeEventListener('offline', handleOffline);
    };
  }, []);

  return {
    isOnline,
    isSyncing: false,
    syncQueueCount: 0,
  };
}
