import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '../../components/ui/EmptyState';

export const Route = createFileRoute('/_app/students')({
  component: StudentsPage,
});

function StudentsPage() {
  return <EmptyState message="empty.noStudents" />;
}
