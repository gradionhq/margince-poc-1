import type { ReactNode } from "react";
import { Navigate } from "react-router-dom";
import { useAuthStore } from "../features/identity/store/authStore.js";

interface ProtectedRouteProps {
  children: ReactNode;
}

export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const { user, loading } = useAuthStore();

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <p className="text-gf-body text-gf-secondary">Loading…</p>
      </div>
    );
  }

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}
