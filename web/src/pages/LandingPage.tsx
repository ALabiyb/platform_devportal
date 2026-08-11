// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useSetupStatus } from "@/lib/api";

// ── Floating service ecosystem (hero section) ──────────────────────────────────

const ECOSYSTEM = [
  { label: "SCM",        icon: "🐙", sub: "Source control",    top: "6%",  left: "4%",  anim: "floatA 5s ease-in-out infinite",   delay: "0s" },
  { label: "Jenkins",    icon: "🤖", sub: "CI/CD pipeline",    top: "4%",  left: "57%", anim: "floatB 6.2s ease-in-out infinite", delay: "0.8s" },
  { label: "Harbor",     icon: "⚓", sub: "Container registry", top: "43%", left: "2%",  anim: "floatC 4.8s ease-in-out infinite", delay: "0.4s" },
  { label: "DefectDojo", icon: "🛡️", sub: "Security hub",      top: "40%", left: "60%", anim: "floatA 5.5s ease-in-out infinite", delay: "1.2s" },
  { label: "ArgoCD",     icon: "🔄", sub: "GitOps deploy",     top: "78%", left: "6%",  anim: "floatB 5s ease-in-out infinite",   delay: "0.2s" },
  { label: "Kubernetes", icon: "☸️", sub: "dev · uat · prod",  top: "76%", left: "56%", anim: "floatC 6s ease-in-out infinite",   delay: "1s" },
];

// ── Service mesh topology (demo form-phase right panel) ────────────────────────

const CX = 245, CY = 152, RADIUS = 112;

const TOPO = [
  { id: "gitea",      icon: "🐙", label: "SCM",        sub: "Source control",    color: "#60a5fa", angle: -90,  steps: [2, 3, 4, 10, 11] },
  { id: "jenkins",    icon: "🤖", label: "Jenkins",    sub: "CI/CD pipeline",    color: "#fb923c", angle: -30,  steps: [5, 6, 7] },
  { id: "harbor",     icon: "⚓", label: "Harbor",     sub: "Container registry", color: "#34d399", angle: 30,   steps: [8, 9] },
  { id: "kubernetes", icon: "☸️", label: "Kubernetes", sub: "dev · uat · prod",  color: "#38bdf8", angle: 90,   steps: [11, 12, 13, 14] },
  { id: "argocd",     icon: "🔄", label: "ArgoCD",     sub: "GitOps deploy",     color: "#f472b6", angle: 150,  steps: [12, 13, 14] },
  { id: "defectdojo", icon: "🛡️", label: "DefectDojo", sub: "Security hub",      color: "#a78bfa", angle: -150, steps: [0, 1] },
];

const STEP_MS = [1200, 900, 800, 1400, 600, 700, 1600, 500, 900, 1000, 700, 1300, 800, 700, 1100];

const TOPO_LABELS = [
  "Ensuring DefectDojo product",
  "Creating CI/CD engagement",
  "Creating source repository",
  "Committing Jenkinsfile + Dockerfile",
  "Creating dev / uat / prod branches",
  "Ensuring Jenkins team folder",
  "Creating multibranch pipeline",
  "Recording Jenkins job URL",
  "Ensuring Harbor registry project",
  "Creating Harbor robot account",
  "Configuring repository webhook",
  "Creating manifest repository",
  "Creating ArgoCD app — dev",
  "Creating ArgoCD app — uat",
  "Creating ArgoCD app — prod + databases",
];

function nodeCenter(angle: number) {
  return {
    x: CX + RADIUS * Math.cos((angle * Math.PI) / 180),
    y: CY + RADIUS * Math.sin((angle * Math.PI) / 180),
  };
}

// ── Interactive provisioning demo ──────────────────────────────────────────────

const DEMO_STEP_TEMPLATES = [
  "Ensure DefectDojo product for {SVC}",
  "Create DefectDojo CI/CD engagement — {SVC} CI/CD",
  "Create source repository: {TEAM}/{SVC}",
  "Commit Jenkinsfile + Dockerfile + VERSION → main; create dev/uat/prod branches",
  "Protect main branch (restrict direct push)",
  "Ensure Jenkins team folder: {TEAM}",
  "Create Jenkins multibranch pipeline: {SVC}",
  "Record Jenkins pipeline job URL",
  "Ensure Harbor registry project: {TEAM}",
  "Create Harbor robot account: {SVC}-jenkins",
  "Configure Gitea webhook → Jenkins",
  "Create manifest repo: {TEAM}/{SVC}-k8s; commit dev/uat/prod manifests",
  "Create ArgoCD Application: {SVC}-dev (namespace: {SVC}-dev)",
  "Create ArgoCD Application: {SVC}-uat (namespace: {SVC}-uat)",
  "Create ArgoCD Application: {SVC}-prod; provision databases",
];

const DEMO_LOGS: string[][] = [
  ["DefectDojo: GET /api/v2/products/?name={SVC} → 200 []", "DefectDojo: POST /api/v2/products/ → 201 Created (id=42)", "DefectDojo: PATCH /api/v2/products/42/ SLA critical=1d high=7d → 200"],
  ["DefectDojo: POST /api/v2/engagements/ → 201 Created (id=17)", "Engagement ID=17 persisted to DB"],
  ["Gitea: POST /api/v1/orgs/{TEAM}/repos → 201 Created", "Clone URL: http://gitea/{TEAM}/{SVC}.git"],
  ["Gitea: PUT /repos/{TEAM}/{SVC}/contents/Jenkinsfile → 201 (sha: a3f7c2b)", "Gitea: PUT /contents/VERSION → 201", "Gitea: PUT /contents/Dockerfile → 201", "Gitea: POST /branches → dev (zero-commit ref from main)", "Gitea: POST /branches → uat", "Gitea: POST /branches → prod"],
  ["Gitea: POST /repos/{TEAM}/{SVC}/branch_protections → 201 (main locked)"],
  ["Jenkins: POST /createItem?name={TEAM} (Folder) → 200 OK"],
  ["Jenkins: POST /job/{TEAM}/createItem?name={SVC} → 200 OK", "Jenkins: scanning branches: main, dev, uat, prod", "Jenkins: Build #1 queued per branch — DEVPORTAL_BOOTSTRAP"],
  ["Job URL: http://jenkins/job/{TEAM}/job/{SVC}/", "Pipeline path recorded in DB"],
  ["Harbor: GET /api/v2.0/projects?name={TEAM} → 404 (not found)", "Harbor: POST /api/v2.0/projects → 201 Created"],
  ["Harbor: POST /api/v2.0/robots → 201 Created", "Robot account: robot${SVC}-jenkins (id=8)", "Credentials stored"],
  ["Gitea: POST /repos/{TEAM}/{SVC}/hooks → 201 Created", "Gitea ping → Jenkins scan → 0 new commits → 0 extra builds"],
  ["Gitea: POST /orgs/{TEAM}/repos → {SVC}-k8s created", "Committing {SVC}/deployment.yaml + service.yaml + ingress.yaml → dev", "Branching uat from dev; committing manifests", "Branching prod from dev; committing manifests"],
  ["ArgoCD: GET /api/v1/applications/{SVC}-dev → 404 (not found)", "ArgoCD: POST /api/v1/applications → 201 Created", "ArgoCD: {SVC}-dev syncing → namespace {SVC}-dev"],
  ["ArgoCD: GET /api/v1/applications/{SVC}-uat → 404 (not found)", "ArgoCD: POST /api/v1/applications → 201 Created", "ArgoCD: {SVC}-uat syncing → namespace {SVC}-uat"],
  ["ArgoCD: GET /api/v1/applications/{SVC}-prod → 404 (not found)", "ArgoCD: POST /api/v1/applications → 201 Created", "Postgres: CREATE DATABASE {SVC_DB}_dev; CREATE USER {SVC_DB}_dev_user", "Postgres: CREATE DATABASE {SVC_DB}_uat; CREATE USER {SVC_DB}_uat_user", "Postgres: CREATE DATABASE {SVC_DB}_prod; CREATE USER {SVC_DB}_prod_user", "✓ {SVC} ready — 3 ArgoCD apps synced (dev · uat · prod)"],
];

type StepStatus = "pending" | "running" | "done";
type LogLine = { text: string; color: string; ts: string };

function logColor(text: string): string {
  if (text.startsWith("✓") || text.includes("→ 201") || text.includes("ready")) return "#34d399";
  if (text.includes("→ 200") || text.includes("→ 204")) return "#86efac";
  if (text.includes("→ 404")) return "#f87171";
  if (/^(Gitea|Jenkins|Harbor|ArgoCD|DefectDojo|Postgres):/.test(text)) return "#60a5fa";
  if (text.startsWith("Branching") || text.startsWith("Committing") || text.startsWith("Clone URL") || text.startsWith("Job URL") || text.startsWith("Robot") || text.startsWith("Engagement")) return "#a78bfa";
  return "#94a3b8";
}

function slugify(v: string): string {
  return v.toLowerCase().replace(/\s+/g, "-").replace(/[^a-z0-9-]/g, "").replace(/-+/g, "-").replace(/^-|-$/g, "") || "my-service";
}

function rStr(s: string, svc: string, team: string): string {
  return s.replace(/{SVC}/g, svc).replace(/{TEAM}/g, team).replace(/{SVC_DB}/g, svc.replace(/-/g, "_"));
}

// ── Features ───────────────────────────────────────────────────────────────────

const FEATURES = [
  { icon: "⚡", title: "Zero-config pipelines", desc: "DevPortal renders, commits, and registers a production-grade Jenkinsfile for every service. Teams get security gates, image signing, SBOM, and DAST — without writing a line of YAML." },
  { icon: "🛡️", title: "Security by default", desc: "Every pipeline includes SAST (SonarQube), SCA (OWASP + OSV), container scanning (Trivy), image signing (Cosign), and DAST (ZAP) — automatically wired to DefectDojo." },
  { icon: "🔀", title: "Multi-env GitOps", desc: "dev → uat → prod promotion without rebuilding. The same image is retagged and promoted. ArgoCD watches the manifest repo and syncs each environment independently." },
  { icon: "🏢", title: "Org hierarchy & RBAC", desc: "Organization → Application → Service hierarchy with team isolation. Developers see only their application's services. Admins get global visibility and credential management." },
  { icon: "📋", title: "Live audit log", desc: "Every create, update, provision, and delete is recorded with the acting user, timestamp, and full diff. Immutable event trail for compliance and incident response." },
  { icon: "🎨", title: "Customizable templates", desc: "Dockerfile and Jenkinsfile templates are editable per build tool from the admin UI. Platform teams can update standards once and all future services inherit the change." },
];

const ROLES = [
  { role: "Developer", icon: "💻", desc: "Create a fully-configured project environment without waiting on DevOps. Your repo, pipeline, registry, and Kubernetes namespaces are ready in under two minutes.", tags: ["git clone ready", "auto pipeline", "image registry"] },
  { role: "DevOps Lead", icon: "⚙️", desc: "Enforce team standards automatically. Every service inherits the correct pipeline template, security gates, namespace conventions, and ArgoCD sync policy.", tags: ["policy enforcement", "template control", "team isolation"] },
  { role: "Admin", icon: "🔑", desc: "Manage users, teams, and platform credentials from one place. Audit every significant action in real time with an immutable event log and full diff history.", tags: ["user management", "audit log", "credential vault"] },
];

const STATS = [
  { value: 15, suffix: "", label: "Automated steps", sub: "per service provisioned" },
  { value: 90, suffix: "s", label: "Average setup time", sub: "repo to running pipeline" },
  { value: 6, suffix: "", label: "Tools integrated", sub: "out of the box" },
  { value: 3, suffix: "", label: "Environments", sub: "dev · uat · prod" },
];

// ── Component ──────────────────────────────────────────────────────────────────

export function LandingPage() {
  const navigate = useNavigate();
  const { data: setup, isLoading: setupLoading } = useSetupStatus();
  const needsSetup = setup?.needs_setup ?? false;

  const primary    = "hsl(var(--primary))";
  const primaryDim = "hsl(var(--brand-hue) 89% 48% / 0.15)";

  // ── Animated counters ───────────────────────────────────────────────────────
  const [counts, setCounts] = useState({ steps: 0, secs: 0, tools: 0, envs: 0 });
  useEffect(() => {
    const targets = { steps: 15, secs: 90, tools: 6, envs: 3 };
    const duration = 1600;
    let start: number | null = null;
    const frame = (ts: number) => {
      if (!start) start = ts;
      const p = Math.min((ts - start) / duration, 1);
      const e = 1 - Math.pow(1 - p, 3);
      setCounts({ steps: Math.round(e * targets.steps), secs: Math.round(e * targets.secs), tools: Math.round(e * targets.tools), envs: Math.round(e * targets.envs) });
      if (p < 1) requestAnimationFrame(frame);
    };
    requestAnimationFrame(frame);
  }, []);

  // ── Topology auto-play ──────────────────────────────────────────────────────
  const [topoStep, setTopoStep]   = useState(0);
  const [packetKey, setPacketKey] = useState(0);
  const topoStepRef = useRef(0);
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout>;
    const advance = () => {
      const s = topoStepRef.current;
      setTopoStep(s);
      setPacketKey(k => k + 1);
      const next = (s + 1) % 15;
      topoStepRef.current = next;
      timer = setTimeout(advance, STEP_MS[next] + 220);
    };
    timer = setTimeout(advance, 700);
    return () => clearTimeout(timer);
  }, []);

  // ── Demo form state ─────────────────────────────────────────────────────────
  const [demoSvc,  setDemoSvc]  = useState("");
  const [demoTeam, setDemoTeam] = useState("");
  const [demoTool, setDemoTool] = useState("maven");
  const [demoActive, setDemoActive]       = useState(false);
  const [demoCurrentStep, setDemoCurrentStep] = useState(-1);
  const [demoStatuses, setDemoStatuses]   = useState<StepStatus[]>(new Array(15).fill("pending"));
  const [demoLogs, setDemoLogs]           = useState<LogLine[]>([]);
  const [demoDone, setDemoDone]           = useState(false);
  const [submittedSvc,  setSubmittedSvc]  = useState("billing-service");
  const [submittedTeam, setSubmittedTeam] = useState("nexbridge");
  const consoleRef  = useRef<HTMLDivElement>(null);
  const stepListRef = useRef<HTMLDivElement>(null);

  const startDemo = () => {
    const svc  = slugify(demoSvc  || "billing-service");
    const team = slugify(demoTeam || "nexbridge");
    setSubmittedSvc(svc);
    setSubmittedTeam(team);
    setDemoActive(true);
    setDemoCurrentStep(-1);
    setDemoStatuses(new Array(15).fill("pending"));
    setDemoLogs([]);
    setDemoDone(false);
  };

  const resetDemo = () => {
    setDemoActive(false);
    setDemoCurrentStep(-1);
    setDemoStatuses(new Array(15).fill("pending"));
    setDemoLogs([]);
    setDemoDone(false);
    setDemoSvc("");
    setDemoTeam("");
    setDemoTool("maven");
  };

  useEffect(() => {
    if (consoleRef.current) consoleRef.current.scrollTop = consoleRef.current.scrollHeight;
  }, [demoLogs]);

  useEffect(() => {
    if (demoCurrentStep >= 0 && stepListRef.current) {
      const el = stepListRef.current.children[demoCurrentStep] as HTMLElement | undefined;
      el?.scrollIntoView({ block: "nearest", behavior: "smooth" });
    }
  }, [demoCurrentStep]);

  useEffect(() => {
    if (!demoActive) return;
    const timers: ReturnType<typeof setTimeout>[] = [];
    let t = 0;
    for (let i = 0; i < 15; i++) {
      const dur = STEP_MS[i], startAt = t;
      timers.push(setTimeout(() => {
        setDemoCurrentStep(i);
        setDemoStatuses(prev => { const n = [...prev]; n[i] = "running"; return n; });
        DEMO_LOGS[i].forEach((log, li) => {
          timers.push(setTimeout(() => {
            const ts = new Date().toLocaleTimeString("en-US", { hour12: false, hour: "2-digit", minute: "2-digit", second: "2-digit" });
            const resolved = rStr(log, submittedSvc, submittedTeam);
            setDemoLogs(prev => [...prev, { text: resolved, color: logColor(resolved), ts }]);
          }, li * 90));
        });
      }, startAt));
      timers.push(setTimeout(() => {
        setDemoStatuses(prev => { const n = [...prev]; n[i] = "done"; return n; });
      }, startAt + dur));
      t += dur + 280;
    }
    timers.push(setTimeout(() => { setDemoCurrentStep(15); setDemoDone(true); }, t));
    return () => timers.forEach(clearTimeout);
  }, [demoActive, submittedSvc, submittedTeam]);

  // ── Topology helpers ────────────────────────────────────────────────────────
  const activeNodeIds = new Set(TOPO.filter(n => n.steps.includes(topoStep)).map(n => n.id));
  const doneNodeIds   = new Set(TOPO.filter(n => n.steps.every(s => s < topoStep)).map(n => n.id));

  // ── Render ──────────────────────────────────────────────────────────────────

  return (
    <div style={{ minWidth: 1024, background: "#0a0f1e", color: "#f8fafc", minHeight: "100vh", fontFamily: "'IBM Plex Sans', system-ui, sans-serif", WebkitFontSmoothing: "antialiased", position: "relative", overflowX: "hidden" }}>

      {/* ── Mesh background ── */}
      <div style={{
        position: "fixed", inset: 0, pointerEvents: "none", zIndex: 0,
        backgroundImage: [
          "radial-gradient(ellipse 60% 50% at 15% 25%, hsl(var(--brand-hue) 89% 48% / 0.08) 0%, transparent 100%)",
          "radial-gradient(ellipse 50% 40% at 85% 70%, rgba(52, 211, 153, 0.05) 0%, transparent 100%)",
          "linear-gradient(rgba(30, 41, 59, 0.45) 1px, transparent 1px)",
          "linear-gradient(90deg, rgba(30, 41, 59, 0.45) 1px, transparent 1px)",
        ].join(", "),
        backgroundSize: "100% 100%, 100% 100%, 52px 52px, 52px 52px",
        WebkitMaskImage: "linear-gradient(to bottom, transparent 0%, black 8%, black 92%, transparent 100%)",
        maskImage: "linear-gradient(to bottom, transparent 0%, black 8%, black 92%, transparent 100%)",
      }} />

      <div style={{ position: "relative", zIndex: 1, maxWidth: 1140, margin: "0 auto", padding: "0 40px" }}>

        {/* ── Nav ── */}
        <header style={{ display: "flex", alignItems: "center", justifyContent: "space-between", height: 68 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <div style={{ width: 32, height: 32, borderRadius: 8, background: `linear-gradient(135deg, ${primary}, hsl(var(--brand-hue) 89% 65%))`, display: "flex", alignItems: "center", justifyContent: "center", fontSize: 14, fontWeight: 800, color: "#fff", boxShadow: `0 0 18px ${primaryDim}` }}>N</div>
            <span style={{ fontSize: 16, fontWeight: 700, letterSpacing: "-0.02em", color: "#f8fafc" }}>DevPortal</span>
            <span style={{ marginLeft: 4, fontSize: 11, fontWeight: 600, color: "#64748b", background: "#1e293b", border: "1px solid #334155", padding: "2px 8px", borderRadius: 20 }}>NexBridge Technologies</span>
          </div>
          <nav style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <button onClick={() => navigate("/login")} className="landing-signin-btn">Sign in</button>
            {!setupLoading && needsSetup && (
              <button onClick={() => navigate("/register")} style={{ height: 36, borderRadius: 6, border: "none", background: primary, color: "#fff", fontSize: 13, fontWeight: 600, padding: "0 18px", cursor: "pointer" }}>Get started</button>
            )}
          </nav>
        </header>

        {/* ── Hero ── */}
        <section style={{ padding: "72px 0 56px", display: "grid", gridTemplateColumns: "1fr 1fr", gap: 48, alignItems: "center", minHeight: 480 }}>
          <div style={{ animation: "slideUpFade 0.7s ease both" }}>
            <div style={{ display: "inline-flex", alignItems: "center", gap: 8, marginBottom: 20, background: "rgba(96,165,250,0.08)", border: "1px solid rgba(96,165,250,0.2)", borderRadius: 20, padding: "5px 14px" }}>
              <span style={{ width: 6, height: 6, borderRadius: "50%", background: "#34d399", display: "inline-block", boxShadow: "0 0 8px #34d399" }} />
              <span style={{ fontSize: 11, fontWeight: 600, color: "#60a5fa", letterSpacing: "0.05em", textTransform: "uppercase" }}>Platform Engineering</span>
            </div>
            <h1 style={{ fontSize: 46, lineHeight: 1.12, fontWeight: 800, letterSpacing: "-0.03em", margin: "0 0 8px", color: "#f8fafc" }}>Ship a service in</h1>
            <h1 className="landing-gradient-text" style={{ fontSize: 46, lineHeight: 1.12, fontWeight: 800, letterSpacing: "-0.03em", margin: "0 0 24px" }}>90 seconds flat.</h1>
            <p style={{ fontSize: 15, lineHeight: 1.7, color: "#94a3b8", margin: "0 0 36px", maxWidth: 460 }}>DevPortal provisions your Git repo, Jenkins pipeline, Harbor registry, security scanning, and Kubernetes environments in a single click — no DevOps knowledge required.</p>
            <div style={{ display: "flex", gap: 12 }}>
              {!setupLoading && needsSetup ? (
                <>
                  <button onClick={() => navigate("/register")} style={{ height: 46, borderRadius: 8, border: "none", background: primary, color: "#fff", fontSize: 14, fontWeight: 700, padding: "0 24px", cursor: "pointer", boxShadow: `0 0 24px ${primaryDim}` }}>Create admin account →</button>
                  <button onClick={() => navigate("/login")} style={{ height: 46, borderRadius: 8, border: "1px solid #334155", background: "transparent", color: "#f8fafc", fontSize: 14, fontWeight: 500, padding: "0 20px", cursor: "pointer" }}>Sign in</button>
                </>
              ) : (
                <button onClick={() => navigate("/login")} style={{ height: 46, borderRadius: 8, border: "none", background: primary, color: "#fff", fontSize: 14, fontWeight: 700, padding: "0 24px", cursor: "pointer", boxShadow: `0 0 24px ${primaryDim}` }}>Sign in to DevPortal →</button>
              )}
            </div>
            {!setupLoading && needsSetup && <p style={{ fontSize: 12, color: "#475569", marginTop: 12 }}>First launch — create the admin account to unlock the platform.</p>}
          </div>

          {/* Floating ecosystem */}
          <div style={{ position: "relative", height: 400, animation: "slideUpFade 0.9s ease 0.2s both" }}>
            <div style={{ position: "absolute", top: "50%", left: "50%", transform: "translate(-50%, -50%)", background: "#0f172a", border: `2px solid ${primary}`, borderRadius: 16, padding: "14px 22px", textAlign: "center", boxShadow: `0 0 40px ${primaryDim}`, zIndex: 10 }}>
              <div style={{ fontSize: 26 }}>🏗️</div>
              <div style={{ fontSize: 13, fontWeight: 700, color: "#f8fafc", marginTop: 4 }}>DevPortal</div>
              <div style={{ fontSize: 10, color: "#60a5fa", marginTop: 2 }}>Orchestrator</div>
            </div>
            <svg style={{ position: "absolute", inset: 0, width: "100%", height: "100%", pointerEvents: "none" }} viewBox="0 0 440 400">
              {[{ x2: 76, y2: 40 }, { x2: 360, y2: 36 }, { x2: 68, y2: 188 }, { x2: 376, y2: 180 }, { x2: 72, y2: 330 }, { x2: 368, y2: 326 }].map((pt, i) => (
                <line key={i} x1="220" y1="200" x2={pt.x2} y2={pt.y2} stroke="rgba(96,165,250,0.15)" strokeWidth="1.5" strokeDasharray="4 4" style={{ animation: "connPulse 3s ease-in-out infinite", animationDelay: `${i * 0.5}s` }} />
              ))}
            </svg>
            {ECOSYSTEM.map(svc => (
              <div key={svc.label} className="landing-badge-float" style={{ position: "absolute", top: svc.top, left: svc.left, background: "#0f172a", border: "1px solid #1e2b3f", borderRadius: 12, padding: "10px 14px", display: "flex", alignItems: "center", gap: 10, animation: svc.anim, animationDelay: svc.delay, boxShadow: "0 4px 24px rgba(0,0,0,0.4)", backdropFilter: "blur(8px)", minWidth: 130 }}>
                <span style={{ fontSize: 22 }}>{svc.icon}</span>
                <div>
                  <div style={{ fontSize: 12, fontWeight: 700, color: "#f8fafc" }}>{svc.label}</div>
                  <div style={{ fontSize: 10, color: "#64748b", marginTop: 1 }}>{svc.sub}</div>
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* ── Stats strip ── */}
        <section style={{ borderTop: "1px solid #1e293b", borderBottom: "1px solid #1e293b", padding: "32px 0", display: "grid", gridTemplateColumns: "repeat(4,1fr)", gap: 1 }}>
          {[
            { val: counts.steps, suffix: "", label: STATS[0].label, sub: STATS[0].sub },
            { val: counts.secs,  suffix: "s", label: STATS[1].label, sub: STATS[1].sub },
            { val: counts.tools, suffix: "", label: STATS[2].label, sub: STATS[2].sub },
            { val: counts.envs,  suffix: "", label: STATS[3].label, sub: STATS[3].sub },
          ].map((s, i) => (
            <div key={i} className="landing-stat-card" style={{ textAlign: "center", padding: "20px 16px", border: "1px solid #1e293b", borderRadius: 12, margin: "0 6px", background: "rgba(15,23,42,0.6)" }}>
              <div style={{ fontSize: 38, fontWeight: 800, letterSpacing: "-0.03em", color: "#f8fafc", lineHeight: 1 }}>{s.val}{s.suffix}</div>
              <div style={{ fontSize: 13, fontWeight: 600, color: "#60a5fa", marginTop: 8 }}>{s.label}</div>
              <div style={{ fontSize: 11, color: "#475569", marginTop: 3 }}>{s.sub}</div>
            </div>
          ))}
        </section>

        {/* ── Interactive provisioning demo ── */}
        <section style={{ padding: "64px 0 56px" }}>
          <div style={{ marginBottom: 36 }}>
            <div style={{ fontSize: 11, fontWeight: 600, letterSpacing: "0.08em", color: "#60a5fa", textTransform: "uppercase", marginBottom: 10 }}>
              {demoActive ? `Provisioning ${submittedSvc} · ${submittedTeam}` : "What happens when you submit"}
            </div>
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
              <h2 style={{ fontSize: 30, fontWeight: 700, letterSpacing: "-0.02em", color: "#f8fafc", margin: 0 }}>15 steps. One click. Zero YAML.</h2>
              {demoActive && (
                <button onClick={resetDemo} style={{ height: 34, borderRadius: 6, border: "1px solid #334155", background: "transparent", color: "#94a3b8", fontSize: 12, fontWeight: 500, padding: "0 14px", cursor: "pointer" }}>
                  🆕 New Service
                </button>
              )}
            </div>
          </div>

          <div style={{ display: "grid", gridTemplateColumns: "1fr 1.4fr", gap: 24, alignItems: "start" }}>

            {/* ── Left: form → step list ── */}
            {!demoActive ? (
              <div style={{ background: "#0b1220", border: "1px solid #1e293b", borderRadius: 14, padding: 28, animation: "slideUpFade 0.4s ease both" }}>
                <div style={{ fontSize: 13, fontWeight: 700, color: "#f8fafc", marginBottom: 20, display: "flex", alignItems: "center", gap: 8 }}>
                  <span style={{ fontSize: 16 }}>🏗️</span> Try the demo
                </div>
                <div style={{ marginBottom: 14 }}>
                  <div style={{ fontSize: 11, color: "#64748b", marginBottom: 5, fontWeight: 500 }}>Service Name</div>
                  <input value={demoSvc} onChange={e => setDemoSvc(e.target.value.toLowerCase().replace(/[^a-z0-9-\s]/g, ""))} placeholder="e.g. billing-service" onKeyDown={e => e.key === "Enter" && startDemo()}
                    style={{ width: "100%", boxSizing: "border-box", background: "#0f172a", border: "1px solid #1e2b3f", borderRadius: 7, padding: "9px 12px", fontSize: 13, color: "#e2e8f0", fontFamily: "'IBM Plex Mono', monospace", outline: "none", transition: "border-color 0.15s" }}
                    onFocus={e => (e.target.style.borderColor = "rgba(96,165,250,0.5)")} onBlur={e => (e.target.style.borderColor = "#1e2b3f")} />
                </div>
                <div style={{ marginBottom: 14 }}>
                  <div style={{ fontSize: 11, color: "#64748b", marginBottom: 5, fontWeight: 500 }}>Team / Namespace</div>
                  <input value={demoTeam} onChange={e => setDemoTeam(e.target.value.toLowerCase().replace(/[^a-z0-9-\s]/g, ""))} placeholder="e.g. nexbridge" onKeyDown={e => e.key === "Enter" && startDemo()}
                    style={{ width: "100%", boxSizing: "border-box", background: "#0f172a", border: "1px solid #1e2b3f", borderRadius: 7, padding: "9px 12px", fontSize: 13, color: "#e2e8f0", fontFamily: "'IBM Plex Mono', monospace", outline: "none", transition: "border-color 0.15s" }}
                    onFocus={e => (e.target.style.borderColor = "rgba(96,165,250,0.5)")} onBlur={e => (e.target.style.borderColor = "#1e2b3f")} />
                </div>
                <div style={{ marginBottom: 22 }}>
                  <div style={{ fontSize: 11, color: "#64748b", marginBottom: 5, fontWeight: 500 }}>Build Tool</div>
                  <select value={demoTool} onChange={e => setDemoTool(e.target.value)} style={{ width: "100%", boxSizing: "border-box", background: "#0f172a", border: "1px solid #1e2b3f", borderRadius: 7, padding: "9px 12px", fontSize: 13, color: "#e2e8f0", outline: "none", cursor: "pointer" }}>
                    <option value="maven">Maven (Java)</option>
                    <option value="gradle">Gradle (Java)</option>
                    <option value="node">Node.js</option>
                    <option value="go">Go</option>
                    <option value="python">Python</option>
                    <option value="dotnet">.NET</option>
                  </select>
                </div>
                <button onClick={startDemo} style={{ width: "100%", height: 42, borderRadius: 8, border: "none", background: primary, color: "#fff", fontSize: 13, fontWeight: 700, cursor: "pointer", boxShadow: `0 0 16px ${primaryDim}` }}
                  onMouseEnter={e => ((e.target as HTMLButtonElement).style.opacity = "0.85")} onMouseLeave={e => ((e.target as HTMLButtonElement).style.opacity = "1")}>
                  Create Service →
                </button>
                <p style={{ fontSize: 11, color: "#334155", textAlign: "center", marginTop: 12 }}>This is a simulated demo — no real resources created</p>
              </div>
            ) : (
              <div style={{ background: "#0b1220", border: "1px solid #1e293b", borderRadius: 14, overflow: "hidden", animation: "slideUpFade 0.35s ease both" }}>
                <div style={{ padding: "14px 18px", borderBottom: "1px solid #1e293b", display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    <span style={{ fontSize: 14 }}>🏗️</span>
                    <span style={{ fontSize: 12, fontWeight: 700, color: "#f8fafc" }}>{demoDone ? "Provisioning complete" : "Provisioning…"}</span>
                  </div>
                  {demoDone && <span style={{ fontSize: 11, fontWeight: 600, color: "#34d399", background: "rgba(52,211,153,0.1)", border: "1px solid rgba(52,211,153,0.25)", borderRadius: 20, padding: "2px 10px" }}>✓ Done</span>}
                </div>
                <div ref={stepListRef} style={{ maxHeight: 380, overflowY: "auto", padding: "6px 0" }}>
                  {DEMO_STEP_TEMPLATES.map((tmpl, i) => {
                    const status = demoStatuses[i];
                    const isRunning = status === "running", isDone = status === "done", isPending = status === "pending";
                    return (
                      <div key={i} style={{ display: "flex", alignItems: "flex-start", gap: 12, padding: "9px 18px", background: isRunning ? "rgba(96,165,250,0.06)" : "transparent", borderLeft: isRunning ? "2px solid rgba(96,165,250,0.6)" : "2px solid transparent", transition: "background 0.2s, border-color 0.2s" }}>
                        <div style={{ width: 22, height: 22, borderRadius: "50%", flexShrink: 0, display: "flex", alignItems: "center", justifyContent: "center", fontSize: 10, fontWeight: 700, marginTop: 1, background: isDone ? "rgba(52,211,153,0.15)" : isRunning ? "rgba(96,165,250,0.15)" : "rgba(71,85,105,0.2)", border: `1.5px solid ${isDone ? "rgba(52,211,153,0.5)" : isRunning ? "rgba(96,165,250,0.5)" : "#334155"}`, color: isDone ? "#34d399" : isRunning ? "#60a5fa" : "#475569", animation: isRunning ? "pulse 1.4s ease-in-out infinite" : "none" }}>
                          {isDone ? "✓" : isRunning ? "▶" : String(i + 1).padStart(2, "0")}
                        </div>
                        <div style={{ flex: 1 }}>
                          <div style={{ fontSize: 11.5, lineHeight: 1.5, color: isDone ? "#94a3b8" : isRunning ? "#e2e8f0" : isPending ? "#475569" : "#64748b", fontFamily: isRunning ? "'IBM Plex Mono', monospace" : "inherit" }}>
                            {rStr(tmpl, submittedSvc, submittedTeam)}
                          </div>
                        </div>
                        {isDone && <span style={{ fontSize: 10, color: "#34d399", flexShrink: 0, marginTop: 3, opacity: 0.7 }}>{(STEP_MS[i] / 1000).toFixed(1)}s</span>}
                      </div>
                    );
                  })}
                </div>
                {demoDone && (
                  <div style={{ borderTop: "1px solid #1e293b", padding: "14px 18px", background: "rgba(52,211,153,0.04)" }}>
                    <div style={{ fontSize: 12, color: "#34d399", fontWeight: 600, marginBottom: 4 }}>✓ {submittedSvc} fully provisioned</div>
                    <div style={{ fontSize: 11, color: "#64748b" }}>repo · pipeline · registry · security · 3 ArgoCD apps · databases</div>
                  </div>
                )}
              </div>
            )}

            {/* ── Right: service mesh topology (inactive) → API console (active) ── */}
            {!demoActive ? (
              /* ── Animated service mesh ── */
              <div style={{ background: "#050b14", border: "1px solid #1e293b", borderRadius: 14, overflow: "hidden", animation: "slideUpFade 0.5s ease 0.1s both", position: "relative" }}>
                {/* Header bar */}
                <div style={{ background: "#0b1220", borderBottom: "1px solid #1e293b", padding: "10px 16px", display: "flex", alignItems: "center", gap: 8 }}>
                  <div style={{ width: 10, height: 10, borderRadius: "50%", background: "#f85149" }} />
                  <div style={{ width: 10, height: 10, borderRadius: "50%", background: "#d29922" }} />
                  <div style={{ width: 10, height: 10, borderRadius: "50%", background: "#3fb950" }} />
                  <span style={{ marginLeft: 8, fontSize: 11, color: "#475569" }}>devportal — service mesh</span>
                  <span style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 5 }}>
                    <span style={{ width: 6, height: 6, borderRadius: "50%", background: "#34d399", display: "inline-block", boxShadow: "0 0 6px #34d399", animation: "pulse 2s ease-in-out infinite" }} />
                    <span style={{ fontSize: 10, color: "#34d399" }}>live</span>
                  </span>
                </div>

                {/* Topology canvas */}
                <div style={{ position: "relative", height: 310, overflow: "hidden" }}>
                  {/* SVG layer: connection lines + animated packets */}
                  <svg viewBox="0 0 490 310" style={{ position: "absolute", inset: 0, width: "100%", height: "100%", pointerEvents: "none" }}>
                    <defs>
                      {TOPO.map(node => (
                        <filter key={node.id} id={`glow-${node.id}`} x="-50%" y="-50%" width="200%" height="200%">
                          <feGaussianBlur stdDeviation="3" result="blur" />
                          <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
                        </filter>
                      ))}
                    </defs>

                    {/* Connection lines */}
                    {TOPO.map(node => {
                      const { x: nx, y: ny } = nodeCenter(node.angle);
                      const isActive = activeNodeIds.has(node.id);
                      const isDone   = doneNodeIds.has(node.id);
                      return (
                        <line key={node.id}
                          x1={CX} y1={CY} x2={nx} y2={ny}
                          stroke={isActive || isDone ? node.color : "#1e293b"}
                          strokeOpacity={isActive ? 0.8 : isDone ? 0.3 : 0.5}
                          strokeWidth={isActive ? 1.5 : 1}
                          strokeDasharray={isDone || isActive ? "none" : "5 5"}
                          style={{ transition: "stroke-opacity 0.5s, stroke-width 0.3s" }}
                        />
                      );
                    })}

                    {/* Animated data packets — SVG animateMotion, keyed to restart per step */}
                    {TOPO.filter(n => activeNodeIds.has(n.id)).map(node => {
                      const { x: nx, y: ny } = nodeCenter(node.angle);
                      const dx = nx - CX, dy = ny - CY;
                      return (
                        <g key={`pkt-${packetKey}-${node.id}`} transform={`translate(${CX}, ${CY})`} filter={`url(#glow-${node.id})`}>
                          <circle r="4.5" fill={node.color}>
                            <animateMotion dur="0.65s" path={`M 0 0 L ${dx} ${dy}`} fill="freeze" />
                            <animate attributeName="opacity" values="1;1;0" keyTimes="0;0.72;1" dur="0.65s" fill="freeze" />
                            <animate attributeName="r" values="4.5;5.5;3" dur="0.65s" fill="freeze" />
                          </circle>
                        </g>
                      );
                    })}
                  </svg>

                  {/* Central DevPortal hub */}
                  <div style={{ position: "absolute", left: CX - 44, top: CY - 36, width: 88, height: 72, background: "#0d1b2e", border: `2px solid ${primary}`, borderRadius: 12, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 3, boxShadow: `0 0 24px ${primaryDim}, 0 0 48px ${primaryDim}`, zIndex: 10 }}>
                    <span style={{ fontSize: 22, lineHeight: 1 }}>🏗️</span>
                    <span style={{ fontSize: 10, fontWeight: 700, color: "#f8fafc" }}>DevPortal</span>
                    <span style={{ fontSize: 9, color: "#60a5fa" }}>Orchestrator</span>
                  </div>

                  {/* Service node cards */}
                  {TOPO.map(node => {
                    const { x: nx, y: ny } = nodeCenter(node.angle);
                    const isActive = activeNodeIds.has(node.id);
                    const isDone   = doneNodeIds.has(node.id);
                    const CW = 90, CH = 56;
                    return (
                      <div key={node.id} style={{
                        position: "absolute",
                        left: nx - CW / 2, top: ny - CH / 2,
                        width: CW, height: CH,
                        background: isActive ? `color-mix(in srgb, ${node.color} 8%, #0a0f1e)` : isDone ? "#0d1320" : "#0a0f1e",
                        border: `1.5px solid ${isActive ? node.color : isDone ? node.color + "44" : "#1e293b"}`,
                        borderRadius: 10,
                        display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 3,
                        boxShadow: isActive
                          ? `0 0 0 3px ${node.color}22, 0 0 20px ${node.color}44, 0 0 40px ${node.color}22`
                          : isDone
                            ? `0 0 8px ${node.color}22`
                            : "none",
                        transition: "border-color 0.4s, box-shadow 0.4s, background 0.4s",
                        zIndex: 5,
                      }}>
                        <span style={{ fontSize: 19, lineHeight: 1, filter: isActive ? `drop-shadow(0 0 6px ${node.color})` : "none", transition: "filter 0.4s" }}>{node.icon}</span>
                        <span style={{ fontSize: 9.5, fontWeight: 700, color: isActive ? "#f8fafc" : isDone ? "#64748b" : "#475569", transition: "color 0.4s" }}>{node.label}</span>
                      </div>
                    );
                  })}
                </div>

                {/* Step label strip */}
                <div style={{ borderTop: "1px solid #0f172a", padding: "10px 18px", background: "#050b14", display: "flex", alignItems: "center", gap: 12 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, flex: 1 }}>
                    <span style={{ fontSize: 10, fontWeight: 700, color: "#334155", fontFamily: "'IBM Plex Mono', monospace", flexShrink: 0 }}>
                      [{String(topoStep + 1).padStart(2, "0")}/15]
                    </span>
                    <span style={{ fontSize: 11, color: "#64748b", fontFamily: "'IBM Plex Mono', monospace", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                      {TOPO_LABELS[topoStep]}
                    </span>
                  </div>
                  {/* Active service pills */}
                  <div style={{ display: "flex", gap: 4, flexShrink: 0 }}>
                    {TOPO.filter(n => activeNodeIds.has(n.id)).map(n => (
                      <span key={n.id} style={{ fontSize: 10, fontWeight: 600, color: n.color, background: n.color + "18", border: `1px solid ${n.color}44`, borderRadius: 12, padding: "1px 7px" }}>{n.label}</span>
                    ))}
                  </div>
                </div>
              </div>
            ) : (
              /* ── Live API console ── */
              <div style={{ background: "#050b14", border: "1px solid #1e293b", borderRadius: 14, overflow: "hidden", fontFamily: "'IBM Plex Mono', monospace", fontSize: 11.5, animation: "slideUpFade 0.35s ease both" }}>
                <div style={{ background: "#0b1220", borderBottom: "1px solid #1e293b", padding: "10px 16px", display: "flex", alignItems: "center", gap: 8 }}>
                  <div style={{ width: 10, height: 10, borderRadius: "50%", background: "#f85149" }} />
                  <div style={{ width: 10, height: 10, borderRadius: "50%", background: "#d29922" }} />
                  <div style={{ width: 10, height: 10, borderRadius: "50%", background: demoDone ? "#3fb950" : "#d29922" }} />
                  <span style={{ marginLeft: 8, fontSize: 11, color: "#475569" }}>devportal — {submittedSvc} · API log</span>
                  {!demoDone && (
                    <span style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 5 }}>
                      <span style={{ width: 6, height: 6, borderRadius: "50%", background: "#60a5fa", display: "inline-block", animation: "cursorBlink 1s step-end infinite" }} />
                      <span style={{ fontSize: 10, color: "#60a5fa" }}>live</span>
                    </span>
                  )}
                </div>
                <div ref={consoleRef} style={{ padding: "12px 16px", height: 380, overflowY: "auto", lineHeight: 1.65 }}>
                  <div style={{ color: "#475569", marginBottom: 8, whiteSpace: "pre" }}>{`$ devportal provision --service ${submittedSvc} --team ${submittedTeam}`}</div>
                  {demoLogs.map((log, i) => (
                    <div key={i} style={{ display: "flex", gap: 8, padding: "1px 0", animation: "termLineIn 0.2s ease both" }}>
                      <span style={{ color: "#334155", flexShrink: 0, userSelect: "none" }}>{log.ts}</span>
                      <span style={{ color: log.color, wordBreak: "break-all" }}>{log.text}</span>
                    </div>
                  ))}
                  {!demoDone && demoLogs.length > 0 && <span style={{ color: "#60a5fa", animation: "cursorBlink 1s step-end infinite" }}>▋</span>}
                  {demoDone && (
                    <div style={{ marginTop: 12, paddingTop: 10, borderTop: "1px solid #1e293b" }}>
                      <div style={{ color: "#34d399", fontWeight: 600, marginBottom: 6 }}>✓ {submittedSvc} ready in {(STEP_MS.reduce((a, b) => a + b, 0) / 1000).toFixed(1)}s</div>
                      <div style={{ color: "#475569" }}>    repo      → {submittedTeam}/{submittedSvc}</div>
                      <div style={{ color: "#475569" }}>    registry  → harbor/{submittedTeam}/{submittedSvc}</div>
                      <div style={{ color: "#475569" }}>    pipeline  → jenkins/{submittedTeam}/{submittedSvc}</div>
                      <div style={{ color: "#475569" }}>    argocd    → 3 apps synced (dev · uat · prod)</div>
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        </section>

        {/* ── Features ── */}
        <section style={{ padding: "8px 0 64px", borderTop: "1px solid #1e293b" }}>
          <div style={{ marginBottom: 40, marginTop: 40 }}>
            <div style={{ fontSize: 11, fontWeight: 600, letterSpacing: "0.08em", color: "#60a5fa", textTransform: "uppercase", marginBottom: 10 }}>Platform capabilities</div>
            <h2 style={{ fontSize: 30, fontWeight: 700, letterSpacing: "-0.02em", color: "#f8fafc", margin: 0 }}>Everything your team needs, built in.</h2>
          </div>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 14 }}>
            {FEATURES.map((f, i) => (
              <div key={f.title} className="landing-feature-card" style={{ border: "1px solid #1e293b", background: "#0b1220", borderRadius: 12, padding: 22, animation: `slideUpFade 0.5s ease ${i * 0.08}s both` }}>
                <div style={{ fontSize: 26, marginBottom: 12 }}>{f.icon}</div>
                <div style={{ fontSize: 14, fontWeight: 700, color: "#f8fafc", marginBottom: 8 }}>{f.title}</div>
                <div style={{ fontSize: 13, color: "#64748b", lineHeight: 1.65 }}>{f.desc}</div>
              </div>
            ))}
          </div>
        </section>

        {/* ── Roles ── */}
        <section style={{ padding: "8px 0 80px", borderTop: "1px solid #1e293b" }}>
          <div style={{ marginBottom: 40, marginTop: 40 }}>
            <div style={{ fontSize: 11, fontWeight: 600, letterSpacing: "0.08em", color: "#60a5fa", textTransform: "uppercase", marginBottom: 10 }}>Built for every role</div>
            <h2 style={{ fontSize: 30, fontWeight: 700, letterSpacing: "-0.02em", color: "#f8fafc", margin: 0 }}>One platform, every stakeholder.</h2>
          </div>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 14 }}>
            {ROLES.map((r, i) => (
              <div key={r.role} className="landing-feature-card" style={{ border: "1px solid #1e293b", background: "#0b1220", borderRadius: 12, padding: 24, animation: `slideUpFade 0.5s ease ${i * 0.1}s both` }}>
                <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 14 }}>
                  <span style={{ fontSize: 22 }}>{r.icon}</span>
                  <span style={{ fontSize: 15, fontWeight: 700, color: "#f8fafc" }}>{r.role}</span>
                </div>
                <p style={{ fontSize: 13, color: "#64748b", lineHeight: 1.65, margin: "0 0 16px" }}>{r.desc}</p>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
                  {r.tags.map(tag => (
                    <span key={tag} style={{ fontSize: 11, fontWeight: 600, color: "#60a5fa", background: "rgba(96,165,250,0.1)", border: "1px solid rgba(96,165,250,0.2)", borderRadius: 20, padding: "3px 10px" }}>{tag}</span>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* ── Footer ── */}
        <footer style={{ borderTop: "1px solid #1e293b", padding: "24px 0", display: "flex", alignItems: "center", justifyContent: "space-between", fontSize: 12, color: "#475569" }}>
          <span>© 2026 NexBridge Technologies · Internal use only</span>
          <span style={{ color: "#334155" }}>DevPortal · Platform Engineering</span>
        </footer>

      </div>
    </div>
  );
}
