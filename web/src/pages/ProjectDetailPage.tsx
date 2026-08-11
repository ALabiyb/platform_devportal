// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useEffect, useRef, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { Download } from "lucide-react";
import { useProject, useProvisioningSteps, useEnvironments, useUpdateService, useReprovisionService } from "@/lib/api";

// ── Types ─────────────────────────────────────────────────────────────────────

type StepStatus = "pending" | "running" | "done" | "failed";

interface StepState {
  step_index: number;
  label: string;
  status: StepStatus;
  detail: string;
}

interface TerminalLine {
  text: string;
  color: string;
}

interface SSEEvent {
  step_index?: number;
  label?: string;
  status?: StepStatus;
  detail?: string;
  done?: boolean;
}

// ── Status helpers ─────────────────────────────────────────────────────────────

function statusColors(status: string) {
  if (status === "active" || status === "done" || status === "Synced")
    return { fg: "#4ade80", bg: "rgba(34,197,94,0.15)", border: "rgba(34,197,94,0.3)" };
  if (status === "provisioning" || status === "running" || status === "OutOfSync" || status === "Progressing")
    return { fg: "#60a5fa", bg: "rgba(59,130,246,0.15)", border: "rgba(59,130,246,0.3)" };
  if (status === "failed")
    return { fg: "#f87171", bg: "rgba(239,68,68,0.15)", border: "rgba(239,68,68,0.3)" };
  return { fg: "#94a3b8", bg: "rgba(148,163,184,0.12)", border: "rgba(148,163,184,0.25)" };
}

function Badge({ status, label }: { status: string; label?: string }) {
  const c = statusColors(status);
  return (
    <span
      style={{ color: c.fg, background: c.bg, border: `1px solid ${c.border}` }}
      className="text-[11px] font-semibold px-2 py-0.5 rounded-full whitespace-nowrap"
    >
      {label ?? status}
    </span>
  );
}

// ── Step status icon (design: small circle with inner indicator) ───────────────

function StepIcon({ status }: { status: StepStatus }) {
  const c = statusColors(status);
  return (
    <span
      style={{ background: c.bg, border: `1px solid ${c.border}` }}
      className="w-5 h-5 rounded-full shrink-0 flex items-center justify-center"
    >
      {status === "done" && (
        <span style={{ color: "#4ade80", fontSize: 11, fontWeight: 700 }}>✓</span>
      )}
      {status === "failed" && (
        <span style={{ color: "#f87171", fontSize: 11, fontWeight: 700 }}>✕</span>
      )}
      {status === "running" && (
        <span
          style={{
            width: 9, height: 9, borderRadius: "50%",
            border: "2px solid #60a5fa", borderTopColor: "transparent",
            display: "block", animation: "spin 0.8s linear infinite",
          }}
        />
      )}
      {status === "pending" && (
        <span style={{ width: 6, height: 6, borderRadius: "50%", background: "#64748b", display: "block" }} />
      )}
    </span>
  );
}

// ── SSE hook ──────────────────────────────────────────────────────────────────

function useProjectStream(projectId: string, initialSteps: StepState[]) {
  const [steps, setSteps] = useState<StepState[]>([]);
  const [termLines, setTermLines] = useState<TerminalLine[]>([]);
  const [finished, setFinished] = useState(false);
  const initialized = useRef(false);

  function reset() {
    initialized.current = false;
    setSteps([]);
    setTermLines([]);
    setFinished(false);
  }

  useEffect(() => {
    if (initialSteps.length > 0 && !initialized.current) {
      initialized.current = true;
      setSteps(initialSteps);
      // build initial terminal from done steps
      const lines: TerminalLine[] = initialSteps
        .filter((s) => s.status === "done" || s.status === "failed")
        .map((s) => ({
          text: `${s.status === "done" ? "✓" : "✗"} ${s.label}`,
          color: s.status === "done" ? "#4ade80" : "#f87171",
        }));
      setTermLines(lines);
      if (initialSteps.every((s) => s.status === "done" || s.status === "failed")) {
        setFinished(true);
      }
    }
  }, [initialSteps]);

  useEffect(() => {
    if (!projectId || finished) return;

    const es = new EventSource(`/api/v1/projects/${projectId}/stream`);

    es.onmessage = (e: MessageEvent) => {
      try {
        const event: SSEEvent = JSON.parse(e.data);

        if (event.done) {
          setFinished(true);
          es.close();
          return;
        }

        if (event.step_index) {
          setSteps((prev) => {
            const next = [...prev];
            const idx = next.findIndex((s) => s.step_index === event.step_index);
            if (idx >= 0) {
              next[idx] = {
                ...next[idx],
                status: event.status ?? next[idx].status,
                detail: event.detail ?? next[idx].detail,
                label: event.label ?? next[idx].label,
              };
            }
            return next;
          });

          // append terminal line on transition to done/failed/running
          if (event.status === "done" || event.status === "failed") {
            setTermLines((prev) => [
              ...prev,
              {
                text: `${event.status === "done" ? "✓" : "✗"} ${event.label ?? ""}${event.detail ? " — " + event.detail : ""}`,
                color: event.status === "done" ? "#4ade80" : "#f87171",
              },
            ]);
          } else if (event.status === "running") {
            setTermLines((prev) => [
              ...prev,
              { text: `… ${event.label ?? ""}`, color: "#93c5fd" },
            ]);
          }
        }
      } catch {
        // ignore parse errors
      }
    };

    es.onerror = () => { es.close(); };
    return () => { es.close(); };
  }, [projectId, finished]);

  return { steps, termLines, finished, reset };
}

// ── Environment status helper ─────────────────────────────────────────────────

function envStatusFromSteps(steps: StepState[], envIdx: number): string {
  // Matches design: ns=6+i, db=9+i, argo=12+i (0-indexed from 0)
  const ns   = steps[6  + envIdx];
  const db   = steps[9  + envIdx];
  const argo = steps[12 + envIdx];
  if (!ns || !db || !argo) return "pending";
  if ([ns, db, argo].some((s) => s.status === "failed")) return "failed";
  if ([ns, db, argo].every((s) => s.status === "done"))  return "active";
  if ([ns, db, argo].some((s) => s.status === "done" || s.status === "running")) return "provisioning";
  return "pending";
}

// ── Page ──────────────────────────────────────────────────────────────────────

const BUILD_TOOLS = [
  { value: "auto",           label: "Auto-detect" },
  { value: "maven",          label: "Java — Maven" },
  { value: "gradle",         label: "Java — Gradle" },
  { value: "go",             label: "Go" },
  { value: "nodejs-express", label: "Node.js — Express" },
  { value: "nextjs",         label: "Next.js" },
  { value: "python-fastapi", label: "Python — FastAPI" },
  { value: "dotnet",         label: ".NET" },
  { value: "flutter-web",    label: "Flutter Web" },
];

// ── Service mesh topology ──────────────────────────────────────────────────────

const TOPO_CX = 245, TOPO_CY = 155, TOPO_R = 112;

interface TopoNode {
  id: string; icon: string; label: string; sub: string;
  color: string; angle: number; steps: number[];
}

// Maps each platform node to 1-based provisioning step indices
const TOPO_NODES: TopoNode[] = [
  { id: "defectdojo", icon: "🛡️", label: "DefectDojo", sub: "Security hub",       color: "#a78bfa", angle: -90,  steps: [1, 2] },
  { id: "gitea",      icon: "🐙", label: "SCM",        sub: "Source control",     color: "#60a5fa", angle: -30,  steps: [3, 4, 5, 11] },
  { id: "jenkins",    icon: "🤖", label: "Jenkins",    sub: "CI/CD pipeline",     color: "#fb923c", angle: 30,   steps: [6, 7, 8] },
  { id: "harbor",     icon: "⚓", label: "Harbor",     sub: "Container registry", color: "#34d399", angle: 90,   steps: [9, 10] },
  { id: "kubernetes", icon: "☸️", label: "Kubernetes", sub: "Workloads",          color: "#38bdf8", angle: 150,  steps: [12, 13, 14, 15] },
  { id: "argocd",     icon: "🔄", label: "ArgoCD",     sub: "GitOps deploy",      color: "#f472b6", angle: -150, steps: [13, 14, 15] },
];

function topoCenter(angle: number) {
  return {
    x: TOPO_CX + TOPO_R * Math.cos((angle * Math.PI) / 180),
    y: TOPO_CY + TOPO_R * Math.sin((angle * Math.PI) / 180),
  };
}

function topoNodeState(node: TopoNode, steps: StepState[]): "pending" | "active" | "done" | "failed" {
  if (steps.length === 0) return "pending";
  const ns = steps.filter(s => node.steps.includes(s.step_index));
  if (ns.length === 0) return "pending";
  if (ns.some(s => s.status === "failed")) return "failed";
  if (ns.some(s => s.status === "running")) return "active";
  if (ns.every(s => s.status === "done")) return "done";
  if (ns.some(s => s.status === "done")) return "active";
  return "pending";
}

export function ProjectDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { data: project, isLoading, refetch: refetchProject } = useProject(id ?? "");
  const { data: rawSteps } = useProvisioningSteps(id ?? "");
  const { data: envData } = useEnvironments(id ?? "");

  const initialSteps: StepState[] = (rawSteps ?? []).map((s) => ({
    step_index: s.step_index,
    label:      s.label,
    status:     (s.status ?? "pending") as StepStatus,
    detail:     s.detail ?? "",
  }));

  const { steps, finished, reset: resetStream } = useProjectStream(id ?? "", initialSteps);

  const allDone   = steps.length > 0 && steps.every((s) => s.status === "done");
  const anyFailed = steps.some((s) => s.status === "failed");

  // ── Edit + Retry state ────────────────────────────────────────────────────
  const [showEdit, setShowEdit] = useState(false);
  const [editName,          setEditName]          = useState("");
  const [editBuildTool,     setEditBuildTool]     = useState("");
  const [editEmail,         setEditEmail]         = useState("");
  const [editTimezone,      setEditTimezone]      = useState("");
  const [editStagingUrl,    setEditStagingUrl]    = useState("");
  const [editManifestPaths, setEditManifestPaths] = useState("");
  const [editError,         setEditError]         = useState("");

  const updateService  = useUpdateService(project?.application_id ?? "", project?.id ?? "");
  const reprovisionSvc = useReprovisionService(project?.application_id ?? "", project?.id ?? "");

  // Populate edit form when project loads or edit panel opens
  useEffect(() => {
    if (project && showEdit) {
      setEditName(project.name);
      setEditBuildTool(project.build_tool);
      setEditEmail(project.notification_email ?? "");
      setEditTimezone(project.app_timezone ?? "Africa/Dar_es_Salaam");
      setEditStagingUrl(project.staging_url ?? "");
      setEditManifestPaths(project.k8s_manifest_paths ?? "04-deployment.yaml");
    }
  }, [project, showEdit]);

  async function handleRetry() {
    if (!reprovisionSvc) return;
    setEditError("");
    resetStream();
    await reprovisionSvc.mutateAsync();
    refetchProject();
  }

  async function handleSaveAndRetry(e: React.FormEvent) {
    e.preventDefault();
    if (!updateService || !reprovisionSvc) return;
    setEditError("");
    try {
      await updateService.mutateAsync({
        name:               editName,
        build_tool:         editBuildTool,
        notification_email: editEmail,
        app_timezone:       editTimezone,
        staging_url:        editStagingUrl,
        k8s_manifest_paths: editManifestPaths,
      });
      setShowEdit(false);
      resetStream();
      await reprovisionSvc.mutateAsync();
      refetchProject();
    } catch (err: unknown) {
      setEditError(err instanceof Error ? err.message : "Failed to save changes.");
    }
  }

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <span
          style={{ width: 28, height: 28, borderRadius: "50%", border: "3px solid #60a5fa", borderTopColor: "transparent", display: "block", animation: "spin 0.8s linear infinite" }}
        />
      </div>
    );
  }

  if (!project) {
    return (
      <div className="p-8">
        <p className="text-sm text-[#94a3b8]">Project not found.</p>
        <Link to="/" className="text-sm text-primary underline mt-2 inline-block">
          Back to dashboard
        </Link>
      </div>
    );
  }

  const envNames = ["dev", "uat", "prod"] as const;

  return (
    <div className="p-8 max-w-[1200px]">
      {/* Header */}
      <Link
        to={project.application_id ? `/applications/${project.application_id}` : "/"}
        className="text-[12px] text-[#94a3b8] no-underline hover:text-[#f8fafc]"
      >
        ← Application
      </Link>
      <div className="flex items-center gap-3 mt-3 mb-1">
        <h1 className="text-[24px] font-bold tracking-tight m-0">{project.name}</h1>
        <Badge status={project.status} />
      </div>
      <p className="text-[13px] text-[#94a3b8] mb-7 font-mono">
        {project.build_tool} · {project.slug ?? project.name.toLowerCase().replace(/\s+/g, "-")} ·{" "}
        {new Date(project.created_at).toLocaleDateString()}
      </p>

      {/* Provisioning section */}
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-[13px] font-semibold tracking-[0.04em] text-[#94a3b8] uppercase m-0">
          Provisioning
        </h2>
        <div className="flex items-center gap-2">
          {anyFailed && (finished || project.status === "failed") && (
            <>
              <button
                onClick={() => setShowEdit((v) => !v)}
                className="text-[12px] text-[#cbd5e1] border border-[#334155] rounded-md px-3 py-1.5 bg-transparent cursor-pointer hover:border-primary/40 transition-colors"
              >
                {showEdit ? "Cancel edit" : "Edit & Retry"}
              </button>
              {!showEdit && (
                <button
                  onClick={handleRetry}
                  disabled={reprovisionSvc.isPending}
                  className="text-[12px] text-white border-none rounded-md px-3 py-1.5 bg-primary cursor-pointer hover:opacity-90 transition-opacity disabled:opacity-50"
                >
                  {reprovisionSvc.isPending ? "Starting…" : "Retry"}
                </button>
              )}
            </>
          )}
          {(allDone || finished || project.status === "active") && !anyFailed && (
            <a
              href={`/api/v1/projects/${project.id}/jenkinsfile`}
              download="Jenkinsfile"
              className="flex items-center gap-1.5 text-[12px] text-[#cbd5e1] border border-[#334155] rounded-md px-3 py-1.5 no-underline hover:border-primary/40 transition-colors"
            >
              <Download className="w-3.5 h-3.5" />
              Jenkinsfile
            </a>
          )}
        </div>
      </div>

      {/* Two-column: steps + topology */}
      <div className="grid gap-5 mb-9" style={{ gridTemplateColumns: "minmax(0, 1.1fr) minmax(0, 1fr)" }}>
        {/* Steps list */}
        <div className="border border-[#334155] bg-[#1e293b] rounded-[10px] p-2">
          {steps.length === 0 ? (
            <div className="py-6 px-4 text-[13px] text-[#64748b]">Waiting for steps…</div>
          ) : (
            steps.map((step) => (
              <div key={step.step_index} className="flex items-start gap-3 px-3 py-[9px] rounded-md">
                <StepIcon status={step.status} />
                <span className="text-[11px] text-[#64748b] font-mono w-[18px] shrink-0 mt-0.5">
                  {step.step_index}
                </span>
                <div className="flex flex-col min-w-0">
                  <span
                    className="text-[13px]"
                    style={{ color: step.status === "pending" ? "#64748b" : "#f8fafc" }}
                  >
                    {step.label}
                  </span>
                  {step.detail && step.status === "failed" && (
                    <span className="text-[11px] font-mono text-[#f87171] truncate mt-0.5">{step.detail}</span>
                  )}
                </div>
              </div>
            ))
          )}
        </div>

        {/* Service mesh topology */}
        <div className="border border-[#1e3a5f] bg-[#060f1e] rounded-[10px] p-4 flex flex-col" style={{ minHeight: 380 }}>
          {/* Status strip */}
          <div className="flex items-center justify-between mb-1">
            <span className="text-[10px] font-mono tracking-widest text-[#334155] uppercase">Platform topology</span>
            <span className="text-[11px] font-mono" style={{
              color: allDone ? "#4ade80" : anyFailed ? "#f87171" : steps.some(s => s.status === "running") ? "#93c5fd" : "#475569",
            }}>
              {allDone
                ? "✓ Complete"
                : anyFailed
                ? "✗ Failed"
                : steps.some(s => s.status === "running")
                ? "Provisioning…"
                : "Pending"}
            </span>
          </div>

          <svg viewBox="0 0 490 334" style={{ width: "100%", flex: 1 }} overflow="visible">
            <defs>
              <filter id="topo-hub-glow" x="-60%" y="-60%" width="220%" height="220%">
                <feGaussianBlur stdDeviation="9" result="blur" />
                <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
              </filter>
              {TOPO_NODES.map(n => (
                <filter key={n.id} id={`tglow-${n.id}`} x="-70%" y="-70%" width="240%" height="240%">
                  <feGaussianBlur stdDeviation="6" result="blur" />
                  <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
                </filter>
              ))}
            </defs>

            {/* Connection lines hub → nodes */}
            {TOPO_NODES.map(n => {
              const { x, y } = topoCenter(n.angle);
              const ns = topoNodeState(n, steps);
              return (
                <line key={n.id}
                  x1={TOPO_CX} y1={TOPO_CY} x2={x} y2={y}
                  stroke={ns === "failed" ? "#f87171" : (ns === "done" || ns === "active") ? n.color : "#0f2a47"}
                  strokeWidth={ns === "pending" ? 1 : 1.5}
                  strokeOpacity={ns === "pending" ? 0.35 : ns === "active" ? 0.75 : ns === "done" ? 0.45 : 0.35}
                  strokeDasharray={ns === "pending" ? "3 5" : undefined}
                />
              );
            })}

            {/* Data packets — continuous loop while node is active */}
            {TOPO_NODES.map(n => {
              const ns = topoNodeState(n, steps);
              if (ns !== "active") return null;
              const { x: nx, y: ny } = topoCenter(n.angle);
              return (
                <circle key={`pkt-${n.id}`} r="4.5" fill={n.color} filter={`url(#tglow-${n.id})`}>
                  <animateMotion dur="0.85s" path={`M ${TOPO_CX} ${TOPO_CY} L ${nx} ${ny}`} repeatCount="indefinite" />
                  <animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.06;0.88;1" dur="0.85s" repeatCount="indefinite" />
                </circle>
              );
            })}

            {/* Service nodes */}
            {TOPO_NODES.map(n => {
              const { x, y } = topoCenter(n.angle);
              const ns = topoNodeState(n, steps);
              const col = ns === "failed" ? "#f87171" : n.color;
              return (
                <g key={n.id}>
                  {/* Pulsing outer ring while active */}
                  {ns === "active" && (
                    <circle cx={x} cy={y} r={26} fill="none" stroke={col} strokeWidth={1.5} strokeOpacity={0.4}>
                      <animate attributeName="r" values="22;29;22" dur="2.4s" repeatCount="indefinite" />
                      <animate attributeName="stroke-opacity" values="0.6;0.12;0.6" dur="2.4s" repeatCount="indefinite" />
                    </circle>
                  )}

                  {/* Node circle */}
                  <circle cx={x} cy={y} r={22}
                    fill={ns === "pending" ? "#0a1628" : `${col}1e`}
                    stroke={ns === "pending" ? "#1e3a5f" : col}
                    strokeWidth={ns === "active" ? 2 : 1.5}
                    filter={ns === "active" ? `url(#tglow-${n.id})` : undefined}
                  />

                  {/* Icon */}
                  <text x={x} y={y} textAnchor="middle" dominantBaseline="central" fontSize={14} style={{ userSelect: "none" }}>
                    {n.icon}
                  </text>

                  {/* Done / failed badge */}
                  {ns === "done" && (
                    <>
                      <circle cx={x + 16} cy={y - 16} r={7} fill="#060f1e" stroke="#4ade80" strokeWidth={1.2} />
                      <text x={x + 16} y={y - 16} textAnchor="middle" dominantBaseline="central" fontSize={8} fill="#4ade80" fontWeight="700">✓</text>
                    </>
                  )}
                  {ns === "failed" && (
                    <>
                      <circle cx={x + 16} cy={y - 16} r={7} fill="#060f1e" stroke="#f87171" strokeWidth={1.2} />
                      <text x={x + 16} y={y - 16} textAnchor="middle" dominantBaseline="central" fontSize={8} fill="#f87171" fontWeight="700">✕</text>
                    </>
                  )}

                  {/* Label */}
                  <text x={x} y={y + 30} textAnchor="middle" fontSize={9.5}
                    fill={ns === "pending" ? "#334155" : "#cbd5e1"}
                    fontWeight={ns === "active" ? 600 : 400}
                  >
                    {n.label}
                  </text>
                  <text x={x} y={y + 42} textAnchor="middle" fontSize={8} fill="#1e3a5f">
                    {n.sub}
                  </text>
                </g>
              );
            })}

            {/* Center hub */}
            {(() => {
              const doneCount = steps.filter(s => s.status === "done").length;
              const total = steps.length || 15;
              const pct = Math.round((doneCount / total) * 100);
              const hubColor = allDone ? "#4ade80" : anyFailed ? "#f87171" : "#60a5fa";
              const isRunning = steps.some(s => s.status === "running");
              return (
                <g filter={allDone || isRunning ? "url(#topo-hub-glow)" : undefined}>
                  <circle cx={TOPO_CX} cy={TOPO_CY} r={37} fill="none" stroke={hubColor} strokeWidth={1} strokeOpacity={0.2} />
                  <circle cx={TOPO_CX} cy={TOPO_CY} r={30}
                    fill={allDone ? "rgba(74,222,128,0.09)" : anyFailed ? "rgba(248,113,113,0.09)" : "rgba(96,165,250,0.07)"}
                    stroke={hubColor} strokeWidth={1.5}
                  />
                  {steps.length > 0 && (
                    <>
                      <text x={TOPO_CX} y={TOPO_CY - 5} textAnchor="middle" dominantBaseline="central"
                        fontSize={14} fill={hubColor} fontWeight="700">
                        {doneCount > 0 || allDone ? `${pct}%` : "⚙"}
                      </text>
                      <text x={TOPO_CX} y={TOPO_CY + 10} textAnchor="middle" fontSize={8.5} fill="#334155">
                        {doneCount}/{total} steps
                      </text>
                    </>
                  )}
                  {steps.length === 0 && (
                    <text x={TOPO_CX} y={TOPO_CY} textAnchor="middle" dominantBaseline="central" fontSize={12} fill="#334155">⚙</text>
                  )}
                </g>
              );
            })()}
          </svg>

          {/* Current / final step label */}
          <div className="mt-2 text-[11px] font-mono text-center min-h-[18px] truncate px-2" style={{
            color: anyFailed && finished ? "#f87171" : allDone ? "#4ade80" : "#64748b",
          }}>
            {steps.some(s => s.status === "running") && `… ${steps.find(s => s.status === "running")?.label}`}
            {allDone && "All provisioning steps completed successfully"}
            {anyFailed && finished && `Failed: ${steps.find(s => s.status === "failed")?.label}`}
          </div>
        </div>
      </div>

      {/* Edit panel — shown when status is failed and user clicks "Edit & Retry" */}
      {showEdit && anyFailed && (
        <form
          onSubmit={handleSaveAndRetry}
          className="border border-[#f87171]/30 bg-[#1e293b] rounded-[10px] p-5 mb-6 flex flex-col gap-4"
        >
          <p className="text-[13px] font-semibold m-0 text-[#f8fafc]">Edit service configuration</p>
          <p className="text-[11px] text-[#64748b] m-0 -mt-2">
            Note: the service slug (<span className="font-mono text-[#93c5fd]">{project.slug}</span>) is fixed — it is baked into the Gitea repo name and Jenkins job.
          </p>

          <div className="grid gap-4" style={{ gridTemplateColumns: "1fr 1fr" }}>
            <div className="flex flex-col gap-1.5">
              <label className="text-[12px] font-medium text-[#94a3b8]">Service name</label>
              <input
                type="text" value={editName} onChange={(e) => setEditName(e.target.value)} required
                className="h-9 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-[inherit] focus:outline-none focus:border-primary/60"
              />
            </div>

            <div className="flex flex-col gap-1.5">
              <label className="text-[12px] font-medium text-[#94a3b8]">Build tool</label>
              <select
                value={editBuildTool} onChange={(e) => setEditBuildTool(e.target.value)}
                className="h-9 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-[inherit] focus:outline-none"
              >
                {BUILD_TOOLS.map((t) => (
                  <option key={t.value} value={t.value}>{t.label}</option>
                ))}
              </select>
            </div>

            <div className="flex flex-col gap-1.5">
              <label className="text-[12px] font-medium text-[#94a3b8]">Notification email</label>
              <input
                type="email" value={editEmail} onChange={(e) => setEditEmail(e.target.value)}
                className="h-9 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-[inherit] focus:outline-none focus:border-primary/60"
              />
            </div>

            <div className="flex flex-col gap-1.5">
              <label className="text-[12px] font-medium text-[#94a3b8]">App timezone</label>
              <input
                type="text" value={editTimezone} onChange={(e) => setEditTimezone(e.target.value)}
                className="h-9 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-mono focus:outline-none focus:border-primary/60"
              />
            </div>

            <div className="flex flex-col gap-1.5">
              <label className="text-[12px] font-medium text-[#94a3b8]">Staging URL <span className="text-[#475569] font-normal">(DAST)</span></label>
              <input
                type="url" value={editStagingUrl} onChange={(e) => setEditStagingUrl(e.target.value)}
                placeholder="https://service-dev.cluster.example.com"
                className="h-9 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-mono focus:outline-none focus:border-primary/60"
              />
            </div>

            <div className="flex flex-col gap-1.5">
              <label className="text-[12px] font-medium text-[#94a3b8]">K8s manifest paths</label>
              <input
                type="text" value={editManifestPaths} onChange={(e) => setEditManifestPaths(e.target.value)}
                className="h-9 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-mono focus:outline-none focus:border-primary/60"
              />
            </div>
          </div>

          {editError && <p className="text-[12px] text-[#f87171] m-0">{editError}</p>}

          <div className="flex gap-3 pt-1">
            <button
              type="submit"
              disabled={updateService.isPending || reprovisionSvc.isPending}
              className="h-9 px-5 rounded-md bg-primary border-none text-white text-[13px] font-medium cursor-pointer disabled:opacity-50"
            >
              {updateService.isPending || reprovisionSvc.isPending ? "Saving…" : "Save & Retry"}
            </button>
            <button
              type="button" onClick={() => setShowEdit(false)}
              className="h-9 px-5 rounded-md border border-[#334155] bg-transparent text-[#94a3b8] text-[13px] cursor-pointer"
            >
              Cancel
            </button>
          </div>
        </form>
      )}

      {/* Environments */}
      <h2 className="text-[13px] font-semibold tracking-[0.04em] text-[#94a3b8] uppercase mb-3">
        Environments
      </h2>
      <div className="grid grid-cols-3 gap-4">
        {envNames.map((envName, i) => {
          const dbEnv = envData?.find((e) => e.name === envName);
          const status = dbEnv?.status ?? (steps.length > 0 ? envStatusFromSteps(steps, i) : "pending");
          const namespace = dbEnv?.namespace ?? `${project.name}-${envName}`;
          const argoStatus = status === "active" ? "Synced" : status === "failed" ? "Unknown" : status === "provisioning" ? "Progressing" : "—";
          const ingressUrl = dbEnv?.ingress_url;
          const dbName = dbEnv?.db_name;

          return (
            <div key={envName} className="border border-[#334155] bg-[#1e293b] rounded-[10px] p-[18px]">
              <div className="flex items-center justify-between mb-3.5">
                <span className="text-[14px] font-semibold uppercase tracking-[0.03em]">
                  {envName}
                </span>
                <Badge status={status} />
              </div>
              <div className="flex flex-col gap-2 text-[12px]">
                <div>
                  <span className="text-[#64748b]">Namespace</span>
                  <div className="font-mono text-[#f8fafc] mt-0.5">{namespace}</div>
                </div>
                <div>
                  <span className="text-[#64748b]">ArgoCD</span>
                  <div className="font-mono text-[#f8fafc] mt-0.5">{argoStatus}</div>
                </div>
                <div>
                  <span className="text-[#64748b]">Database</span>
                  <div className="font-mono text-[#f8fafc] mt-0.5">{dbName ?? "—"}</div>
                </div>
                {ingressUrl && (
                  <div>
                    <span className="text-[#64748b]">Ingress</span>
                    <div className="mt-0.5">
                      <a href={ingressUrl} target="_blank" rel="noreferrer"
                        className="font-mono text-[12px] text-primary no-underline hover:underline">
                        {ingressUrl}
                      </a>
                    </div>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
