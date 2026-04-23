import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '../../../components/ui/EmptyState';

export const Route = createFileRoute('/_app/admin/landing')({
  component: AdminLandingPage,
});

function AdminLandingPage() {
  return <EmptyState message="empty.welcome" />;
}
