// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useNavigate } from "react-router-dom";
import { useApplications } from "@/lib/api";

function StatusDot({ status }: { status: string }) {
  const color = status === "active" ? "#4ade80" : "#f87171";
  return (
    <span style={{ width: 7, height: 7, borderRadius: "50%", background: color, display: "inline-block", marginRight: 6 }} />
  );
}

export function ApplicationsPage() {
  const { data: apps, isLoading } = useApplications();
  const navigate = useNavigate();

  return (
    <div className="p-8 max-w-[1000px]">
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1 className="text-[24px] font-bold tracking-tight m-0 mb-1">Applications</h1>
          <p className="text-[13px] text-[#94a3b8] m-0">
            Business initiatives you are a member of.
          </p>
        </div>
        <button
          onClick={() => navigate("/applications/new")}
          className="h-[36px] rounded-md border-none bg-primary text-white text-[13px] font-medium px-4 cursor-pointer"
        >
          + New application
        </button>
      </div>

      {isLoading ? (
        <div className="text-[13px] text-[#64748b]">Loading…</div>
      ) : !apps?.length ? (
        <div className="border border-dashed border-[#334155] rounded-[10px] py-16 text-center">
          <p className="text-[14px] font-semibold m-0 mb-1">No applications yet</p>
          <p className="text-[12px] text-[#94a3b8] m-0 mb-4">
            Create your first application to start provisioning microservices.
          </p>
          <button
            onClick={() => navigate("/applications/new")}
            className="h-9 px-5 rounded-md bg-primary text-white text-[13px] font-medium border-none cursor-pointer"
          >
            + New application
          </button>
        </div>
      ) : (
        <div className="grid gap-4" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))" }}>
          {apps.map((app) => (
            <div
              key={app.id}
              onClick={() => navigate(`/applications/${app.id}`)}
              className="border border-[#334155] bg-[#1e293b] rounded-[10px] p-5 cursor-pointer hover:border-primary/40 transition-colors"
            >
              <div className="flex items-center justify-between mb-2">
                <span className="text-[15px] font-semibold">{app.name}</span>
                <StatusDot status={app.status} />
              </div>
              {app.description && (
                <p className="text-[12px] text-[#94a3b8] m-0 mb-3 line-clamp-2">{app.description}</p>
              )}
              <p className="text-[11px] text-[#64748b] m-0 font-mono">
                /{app.git_namespace}
              </p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
