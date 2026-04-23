import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '../../../components/ui/EmptyState';

export const Route = createFileRoute('/_app/session/')({
  component: SessionIndexPage,
});

function SessionIndexPage() {
  return <EmptyState message="empty.noSessions" />;
}
