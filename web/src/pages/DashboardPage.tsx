// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// Day 13 will replace this stub with the full project list + create form.
import { useProjects } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { FolderGit2 } from "lucide-react";

function statusVariant(status: string) {
  switch (status) {
    case "provisioned": return "success" as const;
    case "provisioning": return "running" as const;
    case "failed": return "destructive" as const;
    default: return "secondary" as const;
  }
}

export function DashboardPage() {
  const { data: projects, isLoading } = useProjects();

  return (
    <div className="p-6">
      <div className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight">Dashboard</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Overview of your projects — NexBridge DevPortal
        </p>
      </div>

      {isLoading ? (
        <div className="flex items-center gap-2 text-muted-foreground text-sm">
          <div className="h-4 w-4 animate-spin rounded-full border-2 border-primary border-t-transparent" />
          Loading projects…
        </div>
      ) : !projects?.length ? (
        <Card className="flex flex-col items-center justify-center py-16 text-center">
          <FolderGit2 className="h-10 w-10 text-muted-foreground mb-3" />
          <p className="text-sm font-medium">No projects yet</p>
          <p className="text-xs text-muted-foreground mt-1">
            Create your first project to get started.
          </p>
        </Card>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {projects.map((p) => (
            <Card key={p.id} className="hover:border-primary/40 transition-colors cursor-pointer">
              <CardHeader className="pb-2">
                <div className="flex items-start justify-between gap-2">
                  <CardTitle className="text-base">{p.name}</CardTitle>
                  <Badge variant={statusVariant(p.status)}>{p.status}</Badge>
                </div>
              </CardHeader>
              <CardContent>
                <p className="text-xs text-muted-foreground">{p.build_tool}</p>
                <p className="text-xs text-muted-foreground mt-1">
                  {new Date(p.created_at).toLocaleDateString()}
                </p>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
