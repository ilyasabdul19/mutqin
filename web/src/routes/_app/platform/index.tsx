import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '../../../components/ui/EmptyState';

export const Route = createFileRoute('/_app/platform/')({
  component: PlatformIndexPage,
});

function PlatformIndexPage() {
  return <EmptyState message="empty.welcome" />;
}
