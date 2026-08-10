// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { Link } from "react-router-dom";
import { useApplications, useTeams, useProjects } from "@/lib/api";
import { useBrand } from "@/contexts/BrandContext";

function StatCard({ label, value, sub, color }: { label: string; value: number | string; sub?: string; color?: string }) {
  return (
    <div className="border border-[#334155] bg-[#1e293b] rounded-[10px] p-5">
      <p className="text-[12px] text-[#64748b] m-0 mb-1 uppercase tracking-wider">{label}</p>
      <p className={`text-[32px] font-bold m-0 leading-none ${color ?? "text-[#f8fafc]"}`}>{value}</p>
      {sub && <p className="text-[11px] text-[#64748b] m-0 mt-1">{sub}</p>}
    </div>
  );
}

function StatusDot({ status }: { status: string }) {
  const map: Record<string, string> = {
    active: "#4ade80",
    provisioning: "#facc15",
    failed: "#f87171",
    archived: "#475569",
  };
  return (
    <span
      style={{ background: map[status] ?? map.archived }}
      className="inline-block w-2 h-2 rounded-full flex-shrink-0"
    />
  );
}

export function DashboardPage() {
  const { data: applications, isLoading: appsLoading } = useApplications();
  const { data: teams, isLoading: teamsLoading } = useTeams();
  const { data: projects } = useProjects();
  const brand = useBrand();

  const totalApps = applications?.length ?? 0;
  const activeApps = applications?.filter((a) => a.status === "active").length ?? 0;
  const totalServices = projects?.length ?? 0;
  const failedServices = projects?.filter((p) => p.status === "failed").length ?? 0;
  const totalTeams = teams?.length ?? 0;
  const recentApps = [...(applications ?? [])].sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  ).slice(0, 5);

  const isLoading = appsLoading || teamsLoading;

  return (
    <div className="p-8">
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-[24px] font-bold tracking-tight m-0 mb-1">
          {brand.company ? brand.company : "Dashboard"}
        </h1>
        <p className="text-[13px] text-[#64748b] m-0">Internal Developer Platform overview</p>
      </div>

      {isLoading ? (
        <div className="text-[13px] text-[#64748b]">Loading…</div>
      ) : (
        <>
          {/* Stats row */}
          <div className="grid gap-4 mb-8" style={{ gridTemplateColumns: "repeat(3, minmax(180px, 260px))" }}>
            <StatCard label="Applications" value={totalApps} sub={`${activeApps} active`} />
            <StatCard label="Services" value={totalServices}
              sub={failedServices > 0 ? `${failedServices} failed` : "all healthy"}
              color={failedServices > 0 ? "text-[#f87171]" : "text-[#f8fafc]"} />
            <StatCard label="Teams" value={totalTeams} />
          </div>

          <div className="grid gap-6" style={{ gridTemplateColumns: "1fr 320px" }}>
            {/* Recent applications */}
            <div>
              <div className="flex items-center justify-between mb-3">
                <h2 className="text-[14px] font-semibold m-0">Applications</h2>
                <Link to="/applications" className="text-[12px] text-[#60a5fa] no-underline hover:underline">
                  View all →
                </Link>
              </div>

              {!applications?.length ? (
                <div className="border border-dashed border-[#334155] rounded-[10px] py-10 text-center">
                  <p className="text-[13px] font-medium m-0 mb-1">No applications yet</p>
                  <p className="text-[12px] text-[#94a3b8] m-0 mb-4">
                    An application groups related microservices together.
                  </p>
                  <Link to="/applications/new"
                    className="inline-block h-9 px-4 rounded-md bg-primary text-white text-[13px] no-underline leading-9">
                    + New application
                  </Link>
                </div>
              ) : (
                <div className="flex flex-col gap-2">
                  {recentApps.map((app) => (
                    <Link
                      key={app.id}
                      to={`/applications/${app.id}`}
                      className="flex items-center gap-3 border border-[#334155] bg-[#1e293b] rounded-[8px] px-4 py-3 no-underline hover:border-primary/40 transition-colors group"
                    >
                      <StatusDot status={app.status} />
                      <div className="flex-1 min-w-0">
                        <p className="text-[13px] font-medium m-0 text-[#f8fafc] group-hover:text-primary transition-colors truncate">
                          {app.name}
                        </p>
                        <p className="text-[11px] text-[#64748b] m-0 font-mono truncate">/{app.git_namespace}</p>
                      </div>
                      <span className="text-[11px] text-[#475569] flex-shrink-0">
                        {new Date(app.created_at).toLocaleDateString()}
                      </span>
                    </Link>
                  ))}
                  {(applications?.length ?? 0) > 5 && (
                    <Link to="/applications" className="text-[12px] text-[#60a5fa] no-underline text-center py-1 hover:underline">
                      +{(applications?.length ?? 0) - 5} more
                    </Link>
                  )}
                </div>
              )}
            </div>

            {/* Teams panel */}
            <div>
              <div className="flex items-center justify-between mb-3">
                <h2 className="text-[14px] font-semibold m-0">Teams</h2>
                <Link to="/teams" className="text-[12px] text-[#60a5fa] no-underline hover:underline">
                  Manage →
                </Link>
              </div>

              {!teams?.length ? (
                <div className="border border-dashed border-[#334155] rounded-[10px] py-8 text-center">
                  <p className="text-[12px] text-[#94a3b8] m-0 mb-3">No teams yet</p>
                  <Link to="/teams"
                    className="inline-block h-8 px-3 rounded border border-[#334155] text-[#94a3b8] text-[12px] no-underline leading-8 hover:border-primary/40 hover:text-[#f8fafc]">
                    Create team
                  </Link>
                </div>
              ) : (
                <div className="border border-[#334155] rounded-[10px] overflow-hidden">
                  {teams.map((team, i) => (
                    <Link
                      key={team.id}
                      to={`/teams/${team.id}`}
                      className={`flex items-center justify-between px-4 py-3 no-underline hover:bg-[#1e293b]/60 transition-colors group ${i < teams.length - 1 ? "border-b border-[#1e293b]" : ""}`}
                    >
                      <div>
                        <p className="text-[13px] font-medium m-0 text-[#f8fafc] group-hover:text-primary transition-colors">
                          {team.name}
                        </p>
                        <p className="text-[11px] text-[#64748b] m-0 font-mono">{team.slug}</p>
                      </div>
                      <span className="text-[12px] text-[#475569]">→</span>
                    </Link>
                  ))}
                </div>
              )}

              {/* Quick links */}
              <div className="mt-4 flex flex-col gap-2">
                <Link to="/users"
                  className="flex items-center gap-2 text-[12px] text-[#64748b] no-underline hover:text-[#f8fafc] transition-colors px-1">
                  <span>👥</span> User management
                </Link>
                <Link to="/templates"
                  className="flex items-center gap-2 text-[12px] text-[#64748b] no-underline hover:text-[#f8fafc] transition-colors px-1">
                  <span>⚙</span> Pipeline templates
                </Link>
                <Link to="/audit"
                  className="flex items-center gap-2 text-[12px] text-[#64748b] no-underline hover:text-[#f8fafc] transition-colors px-1">
                  <span>📋</span> Audit log
                </Link>
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
