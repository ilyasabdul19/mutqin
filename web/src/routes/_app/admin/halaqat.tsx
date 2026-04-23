import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '../../../components/ui/EmptyState';

export const Route = createFileRoute('/_app/admin/halaqat')({
  component: AdminHalaqatPage,
});

function AdminHalaqatPage() {
  return <EmptyState message="empty.welcome" />;
}
