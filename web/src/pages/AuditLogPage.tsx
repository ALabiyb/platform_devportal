// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useAuditEvents } from "@/lib/api";

interface AuditEvent {
  id: string;
  action: string;
  resource_type: string;
  resource_id?: string;
  created_at: string;
  user_id?: string;
  detail?: Record<string, unknown>;
}

const PAGE_SIZE = 10;

export function AuditLogPage() {
  const { data: events, isLoading } = useAuditEvents();
  const [actionFilter, setActionFilter] = useState("all");
  const [page, setPage] = useState(1);

  const rows = events as AuditEvent[] | undefined;

  const allActions = rows
    ? ["all", ...Array.from(new Set(rows.map((e) => e.action))).sort()]
    : ["all"];

  const filtered = rows
    ? actionFilter === "all"
      ? rows
      : rows.filter((e) => e.action === actionFilter)
    : [];

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const pageRows = filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);

  function setFilter(v: string) {
    setActionFilter(v);
    setPage(1);
  }

  return (
    <div className="p-8 max-w-[1000px]">
      <h1 className="text-[24px] font-bold tracking-tight m-0 mb-1">Audit log</h1>
      <p className="text-[13px] text-[#94a3b8] mb-5">
        Every significant action in DevPortal, recorded and immutable.
      </p>

      <div className="flex gap-2.5 mb-4">
        <select
          value={actionFilter}
          onChange={(e) => setFilter(e.target.value)}
          className="h-[34px] rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-2.5 text-[12px] font-[inherit] focus:outline-none"
        >
          {allActions.map((a) => (
            <option key={a} value={a}>{a}</option>
          ))}
        </select>
      </div>

      {isLoading ? (
        <div className="text-[13px] text-[#64748b]">Loading audit events…</div>
      ) : (
        <>
          <div className="border border-[#334155] bg-[#1e293b] rounded-[10px] overflow-hidden">
            {/* Header */}
            <div
              className="grid px-4 py-2.5 text-[11px] text-[#64748b] uppercase tracking-[0.04em] border-b border-[#334155]"
              style={{ gridTemplateColumns: "160px 1fr 1fr 120px" }}
            >
              <div>Time</div>
              <div>Action</div>
              <div>Resource</div>
              <div>Detail</div>
            </div>

            {pageRows.length === 0 ? (
              <div className="px-4 py-6 text-[13px] text-[#64748b]">No events found.</div>
            ) : (
              pageRows.map((e) => (
                <div
                  key={e.id}
                  className="grid px-4 py-[11px] text-[12px] border-b border-[#334155] last:border-0"
                  style={{ gridTemplateColumns: "160px 1fr 1fr 120px" }}
                >
                  <div className="text-[#94a3b8] font-mono">
                    {new Date(e.created_at).toLocaleString()}
                  </div>
                  <div className="font-mono text-[#93c5fd]">{e.action}</div>
                  <div className="text-[#cbd5e1]">
                    {e.resource_type}
                    {e.resource_id && (
                      <span className="text-[#64748b] ml-1 font-mono text-[11px]">
                        {e.resource_id.slice(0, 8)}
                      </span>
                    )}
                  </div>
                  <div className="text-[#64748b] font-mono text-[11px] truncate">
                    {e.detail ? JSON.stringify(e.detail).slice(0, 40) : "—"}
                  </div>
                </div>
              ))
            )}
          </div>

          <div className="flex items-center justify-end gap-3 mt-3.5">
            <span className="text-[12px] text-[#94a3b8]">
              Page {page} of {totalPages}
            </span>
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page <= 1}
              className="h-[30px] rounded-md border border-[#334155] bg-transparent text-[#cbd5e1] text-[12px] px-3 cursor-pointer disabled:opacity-40"
            >
              Prev
            </button>
            <button
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages}
              className="h-[30px] rounded-md border border-[#334155] bg-transparent text-[#cbd5e1] text-[12px] px-3 cursor-pointer disabled:opacity-40"
            >
              Next
            </button>
          </div>
        </>
      )}
    </div>
  );
}
