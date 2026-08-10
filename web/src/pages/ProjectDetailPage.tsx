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

  const { steps, termLines, finished, reset: resetStream } = useProjectStream(id ?? "", initialSteps);

  // Auto-scroll terminal to bottom
  const termDivRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (termDivRef.current) {
      termDivRef.current.scrollTop = termDivRef.current.scrollHeight;
    }
  }, [termLines]);

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

      {/* Two-column: steps + terminal */}
      <div className="grid gap-5 mb-9" style={{ gridTemplateColumns: "1.1fr 1fr" }}>
        {/* Steps list */}
        <div className="border border-[#334155] bg-[#1e293b] rounded-[10px] p-2">
          {steps.length === 0 ? (
            <div className="py-6 px-4 text-[13px] text-[#64748b]">Waiting for steps…</div>
          ) : (
            steps.map((step) => (
              <div key={step.step_index} className="flex items-center gap-3 px-3 py-[9px] rounded-md">
                <StepIcon status={step.status} />
                <span className="text-[11px] text-[#64748b] font-mono w-[18px] shrink-0">
                  {step.step_index}
                </span>
                <span
                  className="text-[13px]"
                  style={{ color: step.status === "pending" ? "#64748b" : "#f8fafc" }}
                >
                  {step.label}
                </span>
              </div>
            ))
          )}
        </div>

        {/* Terminal log */}
        <div
          ref={termDivRef}
          className="border border-[#334155] bg-[#0b1220] rounded-[10px] p-3.5 font-mono text-[12px] text-[#94a3b8] overflow-y-auto"
          style={{ maxHeight: 420 }}
        >
          <div className="text-[#64748b] text-[11px] mb-2">
            $ devportal provision --project {project.name}
          </div>
          {termLines.length === 0 && (
            <div className="text-[#64748b]">Waiting for output…</div>
          )}
          {termLines.map((line, i) => (
            <div key={i} style={{ color: line.color }} className="py-0.5">
              {line.text}
            </div>
          ))}
          {!finished && steps.some((s) => s.status === "running") && (
            <div className="text-[#93c5fd] animate-pulse">▋</div>
          )}
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
