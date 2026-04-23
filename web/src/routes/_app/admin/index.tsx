import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '../../../components/ui/EmptyState';

export const Route = createFileRoute('/_app/admin/')({
  component: AdminIndexPage,
});

function AdminIndexPage() {
  return <EmptyState message="empty.welcome" />;
}
