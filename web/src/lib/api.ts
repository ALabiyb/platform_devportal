// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useQuery, useMutation } from "@tanstack/react-query";
import { ApiError, queryClient } from "./queryClient";

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json", ...init?.headers },
    ...init,
  });
  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      message = body.error ?? body.message ?? res.statusText;
    } catch {
      message = await res.text().catch(() => res.statusText);
    }
    const err = new ApiError(res.status, message);
    if (res.status === 401) {
      // Session expired — clear auth state so AppRoot redirects to landing page
      // without showing a "something went wrong" error to the user.
      queryClient.setQueryData(["auth", "me"], null);
    }
    throw err;
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

// ── Types ────────────────────────────────────────────────────────────────────

export interface User {
  id: string;
  email: string;
  display_name: string;
  role: string;
  is_active: boolean;
  created_at: string;
}

export interface Project {
  id: string;
  name: string;
  slug: string;
  status: string;
  build_tool: string;
  notification_email: string;
  app_timezone: string;
  staging_url?: string;
  k8s_manifest_paths: string;
  application_id: string;
  git_repo_url?: string;
  created_at: string;
}

export interface CreateProjectResponse {
  id: string;
  name: string;
  slug: string;
  status: string;
  stream_url: string;
  steps_url: string;
}

export interface Application {
  id: string;
  org_id: string;
  name: string;
  slug: string;
  description: string;
  git_namespace: string;
  status: string;
  created_at: string;
}

export interface ApplicationMember {
  application_id: string;
  user_id: string;
  role: string;
  added_at: string;
  email: string;
  display_name: string;
}

export interface Team {
  id: string;
  name: string;
  slug: string;
  org_id: string;
}

export interface TeamMember {
  user_id: string;
  email: string;
  display_name: string;
  role: string;
  member_role: string; // "lead" | "member"
}

export interface Environment {
  id: string;
  project_id: string;
  name: string;
  namespace: string;
  ingress_url: string | null;
  argocd_app_name: string | null;
  db_name: string | null;
  db_username: string | null;
  status: string;
}

export interface ProvisioningStep {
  id: string;
  project_id: string;
  step_index: number;
  label: string;
  status: string;
  detail: string | null;
}

export interface Credential {
  id: string;
  provider_type: string;
  label: string;
  created_at: string;
}

export interface AuditEvent {
  id: string;
  action: string;
  resource_type: string;
  resource_id?: string;
  user_id?: string;
  detail?: Record<string, unknown>;
  created_at: string;
}

// ── Auth ─────────────────────────────────────────────────────────────────────

export function useCurrentUser() {
  return useQuery<User | null>({
    queryKey: ["auth", "me"],
    queryFn: async () => {
      try {
        return await apiFetch<User>("/auth/me");
      } catch (e) {
        if (e instanceof ApiError && e.status === 401) return null;
        throw e;
      }
    },
  });
}

export function useLocalLogin() {
  return useMutation({
    mutationFn: (creds: { email: string; password: string }) =>
      apiFetch<{ token: string }>("/auth/login", {
        method: "POST",
        body: JSON.stringify(creds),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["auth", "me"] });
    },
  });
}

export function useLogout() {
  return useMutation({
    mutationFn: () => apiFetch("/auth/logout", { method: "POST" }),
    onSuccess: () => {
      queryClient.clear();
    },
  });
}

export function useRegister() {
  return useMutation({
    mutationFn: (body: { display_name: string; email: string; password: string }) =>
      apiFetch<User>("/auth/register", {
        method: "POST",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["auth", "me"] });
    },
  });
}

// ── Projects ─────────────────────────────────────────────────────────────────

export function useProjects() {
  return useQuery<Project[]>({
    queryKey: ["projects"],
    queryFn: () => apiFetch("/api/v1/projects"),
  });
}

export function useProject(id: string) {
  return useQuery<Project>({
    queryKey: ["projects", id],
    queryFn: () => apiFetch(`/api/v1/projects/${id}`),
    enabled: !!id,
  });
}

export function useEnvironments(projectId: string) {
  return useQuery<Environment[]>({
    queryKey: ["projects", projectId, "environments"],
    queryFn: () => apiFetch(`/api/v1/projects/${projectId}/environments`),
    enabled: !!projectId,
  });
}

export function useProvisioningSteps(projectId: string) {
  return useQuery<ProvisioningStep[]>({
    queryKey: ["projects", projectId, "steps"],
    queryFn: () => apiFetch(`/api/v1/projects/${projectId}/steps`),
    enabled: !!projectId,
  });
}

export function useCreateProject() {
  return useMutation({
    mutationFn: (body: {
      name: string;
      team_id: string;
      build_tool: string;
      git_namespace: string;
      notification_email: string;
    }) =>
      apiFetch<CreateProjectResponse>("/api/v1/projects", {
        method: "POST",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["projects"] });
    },
  });
}

// ── Applications ──────────────────────────────────────────────────────────────

export function useApplications() {
  return useQuery<Application[]>({
    queryKey: ["applications"],
    queryFn: () => apiFetch("/api/v1/applications"),
  });
}

export function useApplication(id: string) {
  return useQuery<Application>({
    queryKey: ["applications", id],
    queryFn: () => apiFetch(`/api/v1/applications/${id}`),
    enabled: !!id,
  });
}

export function useCreateApplication() {
  return useMutation({
    mutationFn: (body: { name: string; description?: string }) =>
      apiFetch<Application>("/api/v1/applications", { method: "POST", body: JSON.stringify(body) }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["applications"] }),
  });
}

export function useUpdateApplication(appId: string) {
  return useMutation({
    mutationFn: (body: { name: string; description?: string }) =>
      apiFetch<Application>(`/api/v1/applications/${appId}`, { method: "PUT", body: JSON.stringify(body) }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["applications"] });
      queryClient.invalidateQueries({ queryKey: ["applications", appId] });
    },
  });
}

export function useDeleteApplication(appId: string) {
  return useMutation({
    mutationFn: () => apiFetch(`/api/v1/applications/${appId}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["applications"] });
    },
  });
}

export function useApplicationMembers(appId: string) {
  return useQuery<ApplicationMember[]>({
    queryKey: ["applications", appId, "members"],
    queryFn: () => apiFetch(`/api/v1/applications/${appId}/members`),
    enabled: !!appId,
  });
}

export function useAddApplicationMember(appId: string) {
  return useMutation({
    mutationFn: (body: { user_id: string; role: string }) =>
      apiFetch(`/api/v1/applications/${appId}/members`, { method: "POST", body: JSON.stringify(body) }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["applications", appId, "members"] }),
  });
}

export function useRemoveApplicationMember(appId: string) {
  return useMutation({
    mutationFn: (userId: string) =>
      apiFetch(`/api/v1/applications/${appId}/members/${userId}`, { method: "DELETE" }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["applications", appId, "members"] }),
  });
}

export function useApplicationServices(appId: string) {
  return useQuery<Project[]>({
    queryKey: ["applications", appId, "services"],
    queryFn: () => apiFetch(`/api/v1/applications/${appId}/services`),
    enabled: !!appId,
  });
}

export function useDeleteService(appId: string, serviceId: string) {
  return useMutation({
    mutationFn: () =>
      apiFetch(`/api/v1/applications/${appId}/services/${serviceId}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["applications", appId, "services"] });
    },
  });
}

export function useRenameService(appId: string, serviceId: string) {
  return useMutation({
    mutationFn: (body: { name: string }) =>
      apiFetch(`/api/v1/applications/${appId}/services/${serviceId}`, {
        method: "PATCH",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["applications", appId, "services"] });
    },
  });
}

export function useUpdateService(appId: string, serviceId: string) {
  return useMutation({
    mutationFn: (body: {
      name: string;
      build_tool: string;
      notification_email: string;
      app_timezone: string;
      staging_url: string;
      k8s_manifest_paths: string;
    }) =>
      apiFetch<Project>(`/api/v1/applications/${appId}/services/${serviceId}`, {
        method: "PUT",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["projects", serviceId] });
      queryClient.invalidateQueries({ queryKey: ["applications", appId, "services"] });
    },
  });
}

export function useReprovisionService(appId: string, serviceId: string) {
  return useMutation({
    mutationFn: () =>
      apiFetch(`/api/v1/applications/${appId}/services/${serviceId}/reprovision`, {
        method: "POST",
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["projects", serviceId] });
      queryClient.invalidateQueries({ queryKey: ["projects", serviceId, "steps"] });
    },
  });
}

export function useCreateService(appId: string) {
  return useMutation({
    mutationFn: (body: {
      name: string;
      build_tool: string;
      notification_email?: string;
      app_timezone?: string;
      staging_url?: string;
      port?: number;
      health_path?: string;
      vuln_sla_critical?: number;
      vuln_sla_high?: number;
      vuln_sla_medium?: number;
      vuln_sla_low?: number;
      members?: { user_id: string; role: string }[];
      infra_requirements?: { service_type: string; config: Record<string, unknown> }[];
      talks_to?: { project_id: string; port: number }[];
    }) =>
      apiFetch<CreateProjectResponse>(`/api/v1/applications/${appId}/services`, {
        method: "POST",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["applications", appId, "services"] });
    },
  });
}

// ── Pipeline Templates ─────────────────────────────────────────────────────────

export interface PipelineTemplate {
  build_tool: string;
  jenkinsfile: string;
  dockerfile: string;
  updated_at: string;
  updated_by?: string;
}

export function useTemplates() {
  return useQuery<PipelineTemplate[]>({
    queryKey: ["templates"],
    queryFn: () => apiFetch("/api/v1/admin/templates"),
  });
}

export function useTemplate(tool: string) {
  return useQuery<PipelineTemplate>({
    queryKey: ["templates", tool],
    queryFn: () => apiFetch(`/api/v1/admin/templates/${tool}`),
    enabled: !!tool,
  });
}

export function useUpdateTemplate(tool: string) {
  return useMutation({
    mutationFn: (body: { jenkinsfile?: string; dockerfile?: string }) =>
      apiFetch(`/api/v1/admin/templates/${tool}`, {
        method: "PUT",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["templates"] });
      queryClient.invalidateQueries({ queryKey: ["templates", tool] });
    },
  });
}

// ── Teams ─────────────────────────────────────────────────────────────────────

export function useTeams() {
  return useQuery<Team[]>({
    queryKey: ["teams"],
    queryFn: () => apiFetch("/api/v1/teams"),
  });
}

export function useTeam(id: string) {
  return useQuery<Team>({
    queryKey: ["teams", id],
    queryFn: () => apiFetch(`/api/v1/teams/${id}`),
    enabled: !!id,
  });
}

export function useCreateTeam() {
  return useMutation({
    mutationFn: (body: { name: string }) =>
      apiFetch<Team>("/api/v1/teams", { method: "POST", body: JSON.stringify(body) }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["teams"] });
    },
  });
}

export function useUpdateTeam(teamId: string) {
  return useMutation({
    mutationFn: (body: { name: string }) =>
      apiFetch<Team>(`/api/v1/teams/${teamId}`, { method: "PUT", body: JSON.stringify(body) }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["teams"] });
      queryClient.invalidateQueries({ queryKey: ["teams", teamId] });
    },
  });
}

export function useDeleteTeam(teamId: string) {
  return useMutation({
    mutationFn: () => apiFetch(`/api/v1/teams/${teamId}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["teams"] });
    },
  });
}

export function useTeamMembers(teamId: string) {
  return useQuery<TeamMember[]>({
    queryKey: ["teams", teamId, "members"],
    queryFn: () => apiFetch(`/api/v1/teams/${teamId}/members`),
    enabled: !!teamId,
  });
}

export function useAddTeamMember(teamId: string) {
  return useMutation({
    mutationFn: (body: { user_id: string; role: string }) =>
      apiFetch(`/api/v1/teams/${teamId}/members`, {
        method: "POST",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["teams", teamId, "members"] });
    },
  });
}

export function useRemoveTeamMember(teamId: string) {
  return useMutation({
    mutationFn: (userId: string) =>
      apiFetch(`/api/v1/teams/${teamId}/members/${userId}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["teams", teamId, "members"] });
    },
  });
}

// ── Credentials ───────────────────────────────────────────────────────────────

export function useCredentials() {
  return useQuery<Credential[]>({
    queryKey: ["credentials"],
    queryFn: async () => {
      try {
        return await apiFetch("/api/v1/credentials");
      } catch (e) {
        if (e instanceof ApiError && e.status === 501) return [];
        throw e;
      }
    },
  });
}

export function useCreateCredential() {
  return useMutation({
    mutationFn: (body: { provider_type: string; label: string; token: string }) =>
      apiFetch<Credential>("/api/v1/credentials", {
        method: "POST",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["credentials"] });
    },
  });
}

export function useDeleteCredential() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch(`/api/v1/credentials/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["credentials"] });
    },
  });
}

// ── Audit log ─────────────────────────────────────────────────────────────────

export function useAuditEvents() {
  return useQuery<AuditEvent[]>({
    queryKey: ["audit"],
    queryFn: async () => {
      try {
        return await apiFetch("/api/v1/audit");
      } catch (e) {
        if (e instanceof ApiError && e.status === 501) return [];
        throw e;
      }
    },
  });
}

// ── Users (admin) ─────────────────────────────────────────────────────────────

export function useUsers() {
  return useQuery<User[]>({
    queryKey: ["users"],
    queryFn: async () => {
      try {
        return await apiFetch("/api/v1/users");
      } catch (e) {
        if (e instanceof ApiError && e.status === 501) return [];
        throw e;
      }
    },
  });
}

export function useCreateUser() {
  return useMutation({
    mutationFn: (body: { display_name: string; email: string; password: string; role: string }) =>
      apiFetch<User>("/api/v1/users", { method: "POST", body: JSON.stringify(body) }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useUpdateUserRole() {
  return useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: string }) =>
      apiFetch(`/api/v1/users/${userId}/role`, {
        method: "POST",
        body: JSON.stringify({ role }),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useDeactivateUser() {
  return useMutation({
    mutationFn: (userId: string) =>
      apiFetch(`/api/v1/users/${userId}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useSetupStatus() {
  return useQuery<{ needs_setup: boolean }>({
    queryKey: ["setup-status"],
    queryFn: () => apiFetch("/setup/status"),
    staleTime: 30_000,
  });
}

// ── Platform Engineering ───────────────────────────────────────────────────────

export interface Cluster {
  id: string;
  org_id: string;
  name: string;
  display_name: string;
  environment: "dev" | "uat" | "prod";
  api_endpoint: string;
  argocd_url: string;
  kubeconfig_credential_id?: string;
  argocd_credential_id?: string;
  status: string;
  created_at: string;
}

export interface ClusterPlatformService {
  id: string;
  cluster_id: string;
  service_type: string;
  enabled: boolean;
  config: Record<string, string>;
  updated_at: string;
}

export interface ManifestTemplate {
  name: string;
  display_name: string;
  conditional: string;
  content: string;
  updated_at: string;
}

export interface EnvironmentProfile {
  name: string;
  cpu_request: string;
  mem_request: string;
  cpu_limit: string;
  mem_limit: string;
  replicas: number;
  storage_class: string;
  hpa_enabled: boolean;
  hpa_min: number;
  hpa_max: number;
  cpu_threshold: number;
  mem_threshold: number;
  updated_at: string;
}

// Clusters
export function useClusters() {
  return useQuery<Cluster[]>({
    queryKey: ["clusters"],
    queryFn: () => apiFetch("/api/v1/admin/clusters"),
  });
}

export function useCreateCluster() {
  return useMutation({
    mutationFn: (body: Partial<Cluster>) =>
      apiFetch<Cluster>("/api/v1/admin/clusters", { method: "POST", body: JSON.stringify(body) }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["clusters"] }),
  });
}

export function useUpdateCluster(clusterId: string) {
  return useMutation({
    mutationFn: (body: Partial<Cluster>) =>
      apiFetch<Cluster>(`/api/v1/admin/clusters/${clusterId}`, { method: "PUT", body: JSON.stringify(body) }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["clusters"] }),
  });
}

// Cluster Platform Services
export function useClusterServices(clusterId: string) {
  return useQuery<ClusterPlatformService[]>({
    queryKey: ["clusters", clusterId, "services"],
    queryFn: () => apiFetch(`/api/v1/admin/clusters/${clusterId}/services`),
    enabled: !!clusterId,
  });
}

export function useUpsertClusterService(clusterId: string, serviceType: string) {
  return useMutation({
    mutationFn: (body: { enabled: boolean; config: Record<string, string> }) =>
      apiFetch<ClusterPlatformService>(
        `/api/v1/admin/clusters/${clusterId}/services/${serviceType}`,
        { method: "PUT", body: JSON.stringify(body) },
      ),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ["clusters", clusterId, "services"] }),
  });
}

// Manifest Templates
export function useManifestTemplates() {
  return useQuery<ManifestTemplate[]>({
    queryKey: ["manifest-templates"],
    queryFn: () => apiFetch("/api/v1/admin/manifest-templates"),
  });
}

export function useUpsertManifestTemplate(name: string) {
  return useMutation({
    mutationFn: (body: { display_name?: string; conditional?: string; content: string }) =>
      apiFetch<ManifestTemplate>(`/api/v1/admin/manifest-templates/${name}`, {
        method: "PUT",
        body: JSON.stringify(body),
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["manifest-templates"] }),
  });
}

// Environment Profiles
export function useEnvironmentProfiles() {
  return useQuery<EnvironmentProfile[]>({
    queryKey: ["environment-profiles"],
    queryFn: () => apiFetch("/api/v1/admin/environment-profiles"),
  });
}

export function useUpdateEnvironmentProfile(name: string) {
  return useMutation({
    mutationFn: (body: Partial<EnvironmentProfile>) =>
      apiFetch<EnvironmentProfile>(`/api/v1/admin/environment-profiles/${name}`, {
        method: "PUT",
        body: JSON.stringify(body),
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["environment-profiles"] }),
  });
}
