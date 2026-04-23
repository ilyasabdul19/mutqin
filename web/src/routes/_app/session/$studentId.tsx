import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '../../../components/ui/EmptyState';

export const Route = createFileRoute('/_app/session/$studentId')({
  component: SessionStudentPage,
});

function SessionStudentPage() {
  return <EmptyState message="empty.noSessions" />;
}
