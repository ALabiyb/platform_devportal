// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { BrowserRouter, Routes, Route, Navigate, useLocation } from "react-router-dom";
import { Layout } from "@/components/Layout";
import { LandingPage } from "@/pages/LandingPage";
import { LoginPage } from "@/pages/LoginPage";
import { RegisterPage } from "@/pages/RegisterPage";
import { DashboardPage } from "@/pages/DashboardPage";
import { CreateProjectPage } from "@/pages/CreateProjectPage";
import { ProjectDetailPage } from "@/pages/ProjectDetailPage";
import { TeamsPage } from "@/pages/TeamsPage";
import { TeamDetailPage } from "@/pages/TeamDetailPage";
import { ApplicationsPage } from "@/pages/ApplicationsPage";
import { ApplicationDetailPage } from "@/pages/ApplicationDetailPage";
import { CreateApplicationPage } from "@/pages/CreateApplicationPage";
import { CreateServicePage } from "@/pages/CreateServicePage";
import { CredentialsPage } from "@/pages/CredentialsPage";
import { AuditLogPage } from "@/pages/AuditLogPage";
import { UsersPage } from "@/pages/UsersPage";
import { TemplatesPage } from "@/pages/TemplatesPage";
import { PlatformPage } from "@/pages/PlatformPage";
import { useCurrentUser } from "@/lib/api";

function Spinner() {
  return (
    <div className="flex h-screen items-center justify-center bg-[#0f172a]">
      <span
        style={{
          width: 32, height: 32, borderRadius: "50%",
          border: "3px solid #60a5fa", borderTopColor: "transparent",
          display: "block", animation: "spin 0.8s linear infinite",
        }}
      />
    </div>
  );
}

// Shows LandingPage when unauthenticated at "/", redirects to "/" from any
// other path when not logged in. Renders Layout (with Outlet) when authenticated.
function AppRoot() {
  const { data: user, isLoading } = useCurrentUser();
  const location = useLocation();
  if (isLoading) return <Spinner />;
  if (!user) {
    if (location.pathname === "/") return <LandingPage />;
    return <Navigate to="/" state={{ from: location }} replace />;
  }
  return <Layout />;
}

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
        <Route path="/login" element={<PublicOnlyRoute><LoginPage /></PublicOnlyRoute>} />
        <Route path="/register" element={<PublicOnlyRoute><RegisterPage /></PublicOnlyRoute>} />

        <Route path="/" element={<AppRoot />}>
          <Route index element={<DashboardPage />} />
          <Route path="applications" element={<ApplicationsPage />} />
          <Route path="applications/new" element={<CreateApplicationPage />} />
          <Route path="applications/:appId" element={<ApplicationDetailPage />} />
          <Route path="applications/:appId/services/new" element={<CreateServicePage />} />
          <Route path="projects/new" element={<CreateProjectPage />} />
          <Route path="projects/:id" element={<ProjectDetailPage />} />
          <Route path="teams" element={<TeamsPage />} />
          <Route path="teams/:id" element={<TeamDetailPage />} />
          <Route path="credentials" element={<CredentialsPage />} />
          <Route path="audit" element={<AuditLogPage />} />
          <Route path="users" element={<UsersPage />} />
          <Route path="templates" element={<TemplatesPage />} />
          <Route path="platform" element={<PlatformPage />} />
        </Route>

        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
