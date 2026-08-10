// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useApplication, useCreateService, useUsers } from "@/lib/api";
import { ApiError } from "@/lib/queryClient";

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

export function CreateServicePage() {
  const { appId } = useParams<{ appId: string }>();
  const navigate = useNavigate();
  const { data: app } = useApplication(appId ?? "");
  const create = useCreateService(appId ?? "");
  const { data: allUsers = [] } = useUsers();

  const [name, setName] = useState("");
  const [buildTool, setBuildTool] = useState("auto");
  const [email, setEmail] = useState("");
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [timezone, setTimezone] = useState("Africa/Dar_es_Salaam");
  const [stagingUrl, setStagingUrl] = useState("");
  const [manifestPaths, setManifestPaths] = useState("04-deployment.yaml");

  // Security settings
  const [showSecurity, setShowSecurity] = useState(false);
  const [slaC, setSlaC] = useState(7);
  const [slaH, setSlaH] = useState(30);
  const [slaM, setSlaM] = useState(90);
  const [slaL, setSlaL] = useState(180);
  const [selectedMembers, setSelectedMembers] = useState<Set<string>>(new Set());

  const [error, setError] = useState("");

  function toggleMember(userId: string) {
    setSelectedMembers((prev) => {
      const next = new Set(prev);
      if (next.has(userId)) next.delete(userId);
      else next.add(userId);
      return next;
    });
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setError("");
    try {
      const members = Array.from(selectedMembers).map((id) => ({
        user_id: id,
        role: "developer",
      }));
      const svc = await create.mutateAsync({
        name: name.trim(),
        build_tool: buildTool,
        notification_email: email.trim(),
        app_timezone: timezone.trim(),
        staging_url: stagingUrl.trim(),
        k8s_manifest_paths: manifestPaths.trim(),
        vuln_sla_critical: slaC,
        vuln_sla_high: slaH,
        vuln_sla_medium: slaM,
        vuln_sla_low: slaL,
        members,
      });
      navigate(`/projects/${svc.id}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to create service.");
    }
  }

  return (
    <div className="p-8 max-w-[560px]">
      <button
        onClick={() => navigate(`/applications/${appId}`)}
        className="text-[12px] text-[#94a3b8] hover:text-[#f8fafc] bg-transparent border-none cursor-pointer mb-6 block"
      >
        ← Back to {app?.name ?? "Application"}
      </button>

      <div className="border border-[#334155] bg-[#1e293b] rounded-[12px] p-7">
        <div className="flex items-center gap-3 mb-1">
          <div className="w-9 h-9 rounded-lg bg-primary/20 flex items-center justify-center text-primary text-[18px]">
            ⬡
          </div>
          <h1 className="text-[18px] font-bold m-0">Add Service</h1>
        </div>
        {app && (
          <p className="text-[12px] text-[#64748b] mb-5 ml-12">
            Under <span className="text-[#93c5fd] font-mono">/{app.git_namespace}</span>
            {" "}— DevPortal will create the repo, pipeline, registry, and ArgoCD apps automatically.
          </p>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-5">
          {/* ── Core fields ── */}
          <div className="flex flex-col gap-1.5">
            <label className="text-[13px] font-medium">Service name</label>
            <input
              type="text"
              placeholder="e.g. api-gateway"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="h-10 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-[inherit] focus:outline-none focus:border-primary/60"
              autoFocus
            />
            <p className="text-[11px] text-[#64748b] m-0">
              Lowercase letters, numbers, hyphens only.
            </p>
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="text-[13px] font-medium">Build tool</label>
            <select
              value={buildTool}
              onChange={(e) => setBuildTool(e.target.value)}
              className="h-10 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-[inherit] focus:outline-none"
            >
              {BUILD_TOOLS.map((t) => (
                <option key={t.value} value={t.value}>{t.label}</option>
              ))}
            </select>
            <p className="text-[11px] text-[#64748b] m-0">
              Determines the Jenkinsfile template and Docker build strategy.
            </p>
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="text-[13px] font-medium">
              Notification email <span className="text-[#64748b] font-normal">(optional)</span>
            </label>
            <input
              type="email"
              placeholder="team@nexbridge.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="h-10 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-[inherit] focus:outline-none focus:border-primary/60"
            />
          </div>

          {/* ── Security settings ── */}
          <button
            type="button"
            onClick={() => setShowSecurity((v) => !v)}
            className="self-start text-[12px] text-[#64748b] hover:text-[#93c5fd] bg-transparent border-none cursor-pointer p-0 flex items-center gap-1"
          >
            <span>{showSecurity ? "▾" : "▸"}</span>
            Security settings <span className="text-[#475569]">(SLA deadlines, team members)</span>
          </button>

          {showSecurity && (
            <div className="flex flex-col gap-4 pl-4 border-l border-[#1e3a5f]">
              {/* SLA deadlines */}
              <div>
                <p className="text-[13px] font-medium mb-2">CVE remediation SLA (days)</p>
                <p className="text-[11px] text-[#64748b] mb-3 m-0">
                  DefectDojo will flag findings as SLA-breached if not resolved within these deadlines.
                </p>
                <div className="grid grid-cols-2 gap-3">
                  {[
                    { label: "Critical", value: slaC, set: setSlaC, color: "#f87171" },
                    { label: "High",     value: slaH, set: setSlaH, color: "#fb923c" },
                    { label: "Medium",   value: slaM, set: setSlaM, color: "#facc15" },
                    { label: "Low",      value: slaL, set: setSlaL, color: "#4ade80" },
                  ].map(({ label, value, set, color }) => (
                    <div key={label} className="flex flex-col gap-1">
                      <label className="text-[12px] font-medium" style={{ color }}>
                        {label}
                      </label>
                      <div className="flex items-center gap-2">
                        <input
                          type="number"
                          min={1}
                          max={365}
                          value={value}
                          onChange={(e) => set(Math.max(1, Number(e.target.value)))}
                          className="h-9 w-full rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-mono focus:outline-none focus:border-primary/60"
                        />
                        <span className="text-[12px] text-[#64748b] whitespace-nowrap">days</span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              {/* Member assignment */}
              <div>
                <p className="text-[13px] font-medium mb-1">Assign team members</p>
                <p className="text-[11px] text-[#64748b] mb-3 m-0">
                  Only selected members will see this service's findings in DefectDojo.
                  You (the creator) are always added as lead.
                </p>
                {allUsers.length === 0 ? (
                  <p className="text-[12px] text-[#64748b]">No users found.</p>
                ) : (
                  <div className="flex flex-col gap-2 max-h-48 overflow-y-auto pr-1">
                    {allUsers.map((u) => (
                      <label
                        key={u.id}
                        className="flex items-center gap-3 cursor-pointer p-2 rounded-md hover:bg-[#0f172a] transition-colors"
                      >
                        <input
                          type="checkbox"
                          checked={selectedMembers.has(u.id)}
                          onChange={() => toggleMember(u.id)}
                          className="accent-primary w-4 h-4"
                        />
                        <div className="flex flex-col">
                          <span className="text-[13px] text-[#f8fafc]">{u.display_name}</span>
                          <span className="text-[11px] text-[#64748b]">{u.email}</span>
                        </div>
                      </label>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}

          {/* ── Pipeline settings ── */}
          <button
            type="button"
            onClick={() => setShowAdvanced((v) => !v)}
            className="self-start text-[12px] text-[#64748b] hover:text-[#93c5fd] bg-transparent border-none cursor-pointer p-0 flex items-center gap-1"
          >
            <span>{showAdvanced ? "▾" : "▸"}</span>
            Pipeline settings <span className="text-[#475569]">(timezone, DAST, manifests)</span>
          </button>

          {showAdvanced && (
            <div className="flex flex-col gap-4 pl-4 border-l border-[#1e3a5f]">
              <div className="flex flex-col gap-1.5">
                <label className="text-[13px] font-medium">App timezone</label>
                <input
                  type="text"
                  placeholder="Africa/Dar_es_Salaam"
                  value={timezone}
                  onChange={(e) => setTimezone(e.target.value)}
                  className="h-10 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-mono focus:outline-none focus:border-primary/60"
                />
                <p className="text-[11px] text-[#64748b] m-0">
                  Sets <code className="text-[#93c5fd]">APP_TIMEZONE</code> in the Jenkinsfile.
                </p>
              </div>

              <div className="flex flex-col gap-1.5">
                <label className="text-[13px] font-medium">
                  Staging URL <span className="text-[#64748b] font-normal">(enables DAST scanning)</span>
                </label>
                <input
                  type="url"
                  placeholder="https://api-gateway-dev.cluster.example.com"
                  value={stagingUrl}
                  onChange={(e) => setStagingUrl(e.target.value)}
                  className="h-10 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-mono focus:outline-none focus:border-primary/60"
                />
                <p className="text-[11px] text-[#64748b] m-0">
                  Leave empty to skip DAST. Sets <code className="text-[#93c5fd]">STAGING_URL</code>.
                </p>
              </div>

              <div className="flex flex-col gap-1.5">
                <label className="text-[13px] font-medium">K8s manifest paths</label>
                <input
                  type="text"
                  placeholder="04-deployment.yaml"
                  value={manifestPaths}
                  onChange={(e) => setManifestPaths(e.target.value)}
                  className="h-10 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-mono focus:outline-none focus:border-primary/60"
                />
                <p className="text-[11px] text-[#64748b] m-0">
                  Comma-separated. Sets <code className="text-[#93c5fd]">K8S_MANIFEST_PATHS</code>.
                </p>
              </div>
            </div>
          )}

          {error && <p className="text-[12px] text-[#f87171] m-0">{error}</p>}

          <div className="flex gap-3 pt-1">
            <button
              type="submit"
              disabled={!name.trim() || create.isPending}
              className="h-10 flex-1 rounded-md bg-primary border-none text-white text-[13px] font-medium cursor-pointer disabled:opacity-50"
            >
              {create.isPending ? "Creating…" : "Create service"}
            </button>
            <button
              type="button"
              onClick={() => navigate(`/applications/${appId}`)}
              className="h-10 px-5 rounded-md border border-[#334155] bg-transparent text-[#94a3b8] text-[13px] cursor-pointer"
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
