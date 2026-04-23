import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '../../../components/ui/EmptyState';

export const Route = createFileRoute('/_app/session/complete')({
  component: SessionCompletePage,
});

function SessionCompletePage() {
  return <EmptyState message="empty.noSessions" />;
}
