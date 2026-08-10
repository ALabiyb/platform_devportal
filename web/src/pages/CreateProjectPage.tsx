// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import { FolderGit2, ArrowLeft } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { useCreateProject, useTeams } from "@/lib/api";
import { ApiError } from "@/lib/queryClient";

const BUILD_TOOLS = [
  { value: "auto",            label: "Auto-detect" },
  { value: "maven",           label: "Java — Maven" },
  { value: "gradle",          label: "Java — Gradle" },
  { value: "nodejs-express",  label: "Node.js — Express" },
  { value: "nextjs",          label: "Node.js — Next.js" },
  { value: "go",              label: "Go" },
  { value: "python-fastapi",  label: "Python — FastAPI" },
  { value: "dotnet",          label: "C# — .NET" },
  { value: "flutter-web",     label: "Flutter Web" },
];

interface FieldErrors {
  name?: string;
  team_id?: string;
  git_namespace?: string;
  notification_email?: string;
}

function createErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 400) return "Invalid input — check all fields.";
    if (error.status === 409) return "A project with this name already exists in the team.";
  }
  return "Failed to create project. Please try again.";
}

export function CreateProjectPage() {
  const navigate = useNavigate();
  const create = useCreateProject();
  const { data: teams, isLoading: teamsLoading } = useTeams();

  const [name, setName] = useState("");
  const [teamId, setTeamId] = useState("");
  const [buildTool, setBuildTool] = useState("auto");
  const [gitNamespace, setGitNamespace] = useState("");
  const [notificationEmail, setNotificationEmail] = useState("");
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});

  function clearError(field: keyof FieldErrors) {
    setFieldErrors((p) => ({ ...p, [field]: undefined }));
  }

  function validate(): FieldErrors {
    const errs: FieldErrors = {};
    if (!name.trim()) errs.name = "Project name is required.";
    if (!teamId) errs.team_id = "Select a team.";
    if (!gitNamespace.trim()) {
      errs.git_namespace = "Git namespace is required (e.g. nexbridge/backend).";
    }
    if (notificationEmail && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(notificationEmail)) {
      errs.notification_email = "Enter a valid email address.";
    }
    return errs;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const errs = validate();
    if (Object.keys(errs).length > 0) {
      setFieldErrors(errs);
      return;
    }
    setFieldErrors({});

    try {
      const project = await create.mutateAsync({
        name: name.trim(),
        team_id: teamId,
        build_tool: buildTool,
        git_namespace: gitNamespace.trim(),
        notification_email: notificationEmail.trim(),
      });
      navigate(`/projects/${project.id}`);
    } catch {
      // error rendered via create.error below
    }
  }

  return (
    <div className="p-6 max-w-xl mx-auto">
      <div className="mb-6 flex items-center gap-3">
        <Link
          to="/"
          className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          <ArrowLeft className="h-4 w-4" />
          Back
        </Link>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2 mb-1">
            <FolderGit2 className="h-5 w-5 text-primary" />
            <CardTitle className="text-xl">New project</CardTitle>
          </div>
          <CardDescription>
            DevPortal will create the repository, pipeline, registry project,
            and ArgoCD applications automatically.
          </CardDescription>
        </CardHeader>

        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-5" noValidate>
            {/* Project name */}
            <div className="space-y-1.5">
              <Label htmlFor="name">Project name</Label>
              <Input
                id="name"
                placeholder="payment-service"
                value={name}
                onChange={(e) => { setName(e.target.value); clearError("name"); }}
                aria-invalid={!!fieldErrors.name}
              />
              {fieldErrors.name ? (
                <p className="text-xs text-destructive">{fieldErrors.name}</p>
              ) : (
                <p className="text-xs text-muted-foreground">
                  Lowercase letters, numbers, hyphens. Becomes the slug in all tools.
                </p>
              )}
            </div>

            {/* Team */}
            <div className="space-y-1.5">
              <Label htmlFor="team">Team</Label>
              {teamsLoading ? (
                <div className="h-10 rounded-md border border-input bg-background animate-pulse" />
              ) : (
                <Select
                  id="team"
                  value={teamId}
                  onChange={(e) => { setTeamId(e.target.value); clearError("team_id"); }}
                  aria-invalid={!!fieldErrors.team_id}
                >
                  <option value="">Select a team…</option>
                  {teams?.map((t) => (
                    <option key={t.id} value={t.id}>{t.name}</option>
                  ))}
                </Select>
              )}
              {fieldErrors.team_id && (
                <p className="text-xs text-destructive">{fieldErrors.team_id}</p>
              )}
              {!teamsLoading && !teams?.length && (
                <p className="text-xs text-muted-foreground">
                  No teams found — create a team first.
                </p>
              )}
            </div>

            {/* Build tool */}
            <div className="space-y-1.5">
              <Label htmlFor="build-tool">Build tool</Label>
              <Select
                id="build-tool"
                value={buildTool}
                onChange={(e) => setBuildTool(e.target.value)}
              >
                {BUILD_TOOLS.map((bt) => (
                  <option key={bt.value} value={bt.value}>{bt.label}</option>
                ))}
              </Select>
              <p className="text-xs text-muted-foreground">
                Determines the Jenkinsfile template and Docker build strategy.
              </p>
            </div>

            {/* Git namespace */}
            <div className="space-y-1.5">
              <Label htmlFor="git-namespace">GitLab namespace</Label>
              <Input
                id="git-namespace"
                placeholder="nexbridge/backend"
                value={gitNamespace}
                onChange={(e) => { setGitNamespace(e.target.value); clearError("git_namespace"); }}
                aria-invalid={!!fieldErrors.git_namespace}
              />
              {fieldErrors.git_namespace ? (
                <p className="text-xs text-destructive">{fieldErrors.git_namespace}</p>
              ) : (
                <p className="text-xs text-muted-foreground">
                  GitLab group path where the source repository will be created.
                </p>
              )}
            </div>

            {/* Notification email */}
            <div className="space-y-1.5">
              <Label htmlFor="notification-email">
                Notification email{" "}
                <span className="text-muted-foreground font-normal">(optional)</span>
              </Label>
              <Input
                id="notification-email"
                type="email"
                placeholder="team@nexbridge.com"
                value={notificationEmail}
                onChange={(e) => { setNotificationEmail(e.target.value); clearError("notification_email"); }}
                aria-invalid={!!fieldErrors.notification_email}
              />
              {fieldErrors.notification_email && (
                <p className="text-xs text-destructive">{fieldErrors.notification_email}</p>
              )}
            </div>

            {/* Server error */}
            {create.isError && (
              <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {createErrorMessage(create.error)}
              </p>
            )}

            <div className="flex gap-3 pt-1">
              <Button type="submit" disabled={create.isPending} className="flex-1">
                {create.isPending ? (
                  <span className="flex items-center gap-2">
                    <span className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
                    Creating…
                  </span>
                ) : (
                  "Create project"
                )}
              </Button>
              <Button
                type="button"
                variant="outline"
                onClick={() => navigate("/")}
                disabled={create.isPending}
              >
                Cancel
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
