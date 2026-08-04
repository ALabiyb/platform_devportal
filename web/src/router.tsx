// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { BrowserRouter, Routes, Route, Navigate, useLocation } from "react-router-dom";
import { Layout } from "@/components/Layout";
import { LoginPage } from "@/pages/LoginPage";
import { RegisterPage } from "@/pages/RegisterPage";
import { DashboardPage } from "@/pages/DashboardPage";
import { useCurrentUser } from "@/lib/api";

function Spinner() {
  return (
    <div className="flex h-screen items-center justify-center bg-background">
      <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
    </div>
  );
}

// ProtectedRoute: redirects unauthenticated users to /login, preserving the
// intended destination in location.state.from so LoginPage can redirect back.
function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { data: user, isLoading } = useCurrentUser();
  const location = useLocation();

  if (isLoading) return <Spinner />;

  if (!user) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <>{children}</>;
}

// PublicOnlyRoute: redirects already-authenticated users away from /login and
// /register so they never see the auth screens while logged in.
function PublicOnlyRoute({ children }: { children: React.ReactNode }) {
  const { data: user, isLoading } = useCurrentUser();

  if (isLoading) return <Spinner />;

  if (user) return <Navigate to="/" replace />;

  return <>{children}</>;
}

export function Router() {
  return (
    <BrowserRouter>
      <Routes>
        {/* Public auth routes — bounce logged-in users back to "/" */}
        <Route
          path="/login"
          element={
            <PublicOnlyRoute>
              <LoginPage />
            </PublicOnlyRoute>
          }
        />
        <Route
          path="/register"
          element={
            <PublicOnlyRoute>
              <RegisterPage />
            </PublicOnlyRoute>
          }
        />

        {/* Protected layout — all app pages nest here */}
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <Layout />
            </ProtectedRoute>
          }
        >
          <Route index element={<DashboardPage />} />
          {/* Day 13: /projects, /projects/:id */}
          {/* Day 15: /credentials, /audit   */}
        </Route>

        {/* Catch-all — unknown paths fall into the app (React Router handles them) */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
