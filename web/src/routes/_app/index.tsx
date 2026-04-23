import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '../../components/ui/EmptyState';

export const Route = createFileRoute('/_app/')({
  component: HomePage,
});

function HomePage() {
  return <EmptyState message="empty.welcome" />;
}
