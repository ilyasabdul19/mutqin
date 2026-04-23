import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '../../components/ui/EmptyState';

export const Route = createFileRoute('/_app/settings')({
  component: SettingsPage,
});

function SettingsPage() {
  return <EmptyState message="empty.welcome" />;
}
