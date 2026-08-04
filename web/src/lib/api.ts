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
    const text = await res.text().catch(() => res.statusText);
    throw new ApiError(res.status, text);
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
}

export interface Project {
  id: string;
  name: string;
  slug: string;
  status: string;
  build_tool: string;
  created_at: string;
}

export interface Team {
  id: string;
  name: string;
  slug: string;
  org_id: string;
}

export interface ProvisioningStep {
  id: string;
  project_id: string;
  step_index: number;
  label: string;
  status: string;
  detail: string;
  updated_at: string;
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
      apiFetch<Project>("/api/v1/projects", {
        method: "POST",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["projects"] });
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
