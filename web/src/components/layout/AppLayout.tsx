import { Outlet, useMatches } from '@tanstack/react-router';
import { TopBar } from './TopBar';
import { BottomTabBar } from './BottomTabBar';
import { OfflineBanner } from '../ui/OfflineBanner';

const routeTitles: Record<string, string> = {
  '/': 'nav.home',
  '/students': 'nav.students',
  '/session': 'nav.record',
  '/attendance': 'nav.attendance',
  '/settings': 'nav.settings',
  '/admin': 'nav.home',
  '/platform': 'nav.home',
};

export function AppLayout() {
  const matches = useMatches();
  const lastMatch = matches[matches.length - 1];
  const pathname = lastMatch?.pathname ?? '/';
  const title = routeTitles[pathname] ?? 'nav.home';

  return (
    <div className="flex h-dvh flex-col bg-gray-50">
      <TopBar title={title} />
      <OfflineBanner />
      <main className="flex-1 overflow-y-auto">
        <Outlet />
      </main>
      <BottomTabBar />
    </div>
  );
}
