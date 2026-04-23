import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '../../../components/ui/EmptyState';

export const Route = createFileRoute('/_app/platform/organizations')({
  component: PlatformOrganizationsPage,
});

function PlatformOrganizationsPage() {
  return <EmptyState message="empty.welcome" />;
}
