import { createRootRoute, Outlet } from '@tanstack/react-router';
import { useDirection } from '../hooks/useDirection';

function RootComponent() {
  useDirection();
  return <Outlet />;
}

export const Route = createRootRoute({
  component: RootComponent,
});
