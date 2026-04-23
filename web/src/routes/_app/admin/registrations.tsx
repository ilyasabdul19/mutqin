import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '../../../components/ui/EmptyState';

export const Route = createFileRoute('/_app/admin/registrations')({
  component: AdminRegistrationsPage,
});

function AdminRegistrationsPage() {
  return <EmptyState message="empty.welcome" />;
}
