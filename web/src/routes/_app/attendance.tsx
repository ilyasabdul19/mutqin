import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '../../components/ui/EmptyState';

export const Route = createFileRoute('/_app/attendance')({
  component: AttendancePage,
});

function AttendancePage() {
  return <EmptyState message="empty.noAttendance" />;
}
