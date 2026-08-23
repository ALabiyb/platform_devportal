// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { useTheme } from "@/components/Layout";

// ── Hero terminal ─────────────────────────────────────────────────────────────
const CMD = "devportal provision ledger-reconciler --app payments-core";
const LOG_LINES = [
  { mark: "·", text: "resolving language profile", dur: "maven · java 21",     markColor: "var(--faint)" },
  { mark: "✓", text: "git repository created",     dur: "0.6s",                markColor: "var(--ok)" },
  { mark: "✓", text: "scaffold seeded from archetype", dur: "0.9s",             markColor: "var(--ok)" },
  { mark: "✓", text: "jenkins multibranch pipeline registered", dur: "1.2s",    markColor: "var(--ok)" },
  { mark: "✓", text: "harbor project + robot account", dur: "1.5s",             markColor: "var(--ok)" },
  { mark: "✓", text: "kustomize base + 3 overlays generated", dur: "1.8s",      markColor: "var(--ok)" },
  { mark: "✓", text: "postgresql cluster requested (cnpg)", dur: "2.1s",        markColor: "var(--ok)" },
  { mark: "✓", text: "kafka topics applied (strimzi)", dur: "2.4s",              markColor: "var(--ok)" },
  { mark: "✓", text: "argocd application synced", dur: "2.7s",                   markColor: "var(--ok)" },
  { mark: "✓", text: "enrolled in defectdojo + dependencytrack", dur: "3.0s",   markColor: "var(--ok)" },
  { mark: "→", text: "service ready on eu-prod-01", dur: "",                    markColor: "var(--accent)" },
];
const FRAMES_PER_LINE = 11;
const HOLD_FRAMES = 55;
const TOTAL = CMD.length + LOG_LINES.length * FRAMES_PER_LINE + HOLD_FRAMES;

function HeroTerminal({ small = false }: { small?: boolean }) {
  const [frame, setFrame] = useState(0);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    timerRef.current = setInterval(() => {
      setFrame(f => (f + 1) % TOTAL);
    }, small ? 44 : 42);
    return () => { if (timerRef.current) clearInterval(timerRef.current); };
  }, [small]);

  const typedChars = Math.min(frame, CMD.length);
  const linesDone = frame > CMD.length ? Math.min(Math.floor((frame - CMD.length) / FRAMES_PER_LINE), LOG_LINES.length) : 0;
  const progress = Math.round((linesDone / LOG_LINES.length) * 100);

  const mono = small ? 11.5 : 12.5;
  const minH = small ? 186 : 376;

  return (
    <div style={{ background: "var(--panel)", border: "1px solid var(--line)", borderRadius: 12, boxShadow: "0 24px 48px -24px rgba(0,0,0,.5)", overflow: "hidden" }}>
      {/* Title bar */}
      <div style={{ padding: "11px 14px", background: "var(--card)", display: "flex", alignItems: "center", gap: 6 }}>
        {[{ c: "var(--bad)" }, { c: "var(--warn)" }, { c: "var(--ok)" }].map((d, i) => (
          <span key={i} style={{ width: 9, height: 9, borderRadius: "50%", background: d.c, flexShrink: 0 }} />
        ))}
        <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, color: "var(--faint)", marginLeft: 6, flex: 1 }}>devportal — provisioning</span>
        <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, color: "var(--faint)" }}>{(linesDone * 0.3).toFixed(1)}s</span>
      </div>
      {/* Progress rail */}
      <div style={{ height: 2, background: "var(--card)" }}>
        <div style={{ height: "100%", background: "var(--accent)", width: `${progress}%`, transition: "width .25s linear" }} />
      </div>
      {/* Body */}
      <div style={{ padding: small ? "14px 16px 16px" : "18px 20px 22px", fontFamily: "JetBrains Mono,monospace", fontSize: mono, lineHeight: "22px", minHeight: minH }}>
        {/* Command line */}
        <div style={{ marginBottom: 12 }}>
          <span style={{ color: "var(--accent)" }}>$ </span>
          <span style={{ color: "var(--text)" }}>{CMD.slice(0, typedChars)}</span>
          {typedChars < CMD.length && (
            <span style={{ display: "inline-block", width: 7, height: 15, background: "var(--accent)", verticalAlign: "middle", marginLeft: 1, animation: "dcblink 1s step-end infinite" }} />
          )}
        </div>
        {/* Log lines */}
        {LOG_LINES.slice(0, linesDone).map((line, i) => (
          <div key={i} style={{ display: "grid", gridTemplateColumns: "16px minmax(0,1fr) auto", gap: 10, animation: "dcrise .22s ease-out both" }}>
            <span style={{ color: line.markColor }}>{line.mark}</span>
            <span style={{ color: "var(--muted)" }}>{line.text}</span>
            <span style={{ color: "var(--faint)" }}>{line.dur}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Stats counter ─────────────────────────────────────────────────────────────
function CountUp({ target, decimals = 0 }: { target: number; decimals?: number }) {
  const [val, setVal] = useState(0);
  useEffect(() => {
    let t = 0;
    const ticks = 34;
    const timer = setInterval(() => {
      t++;
      const p = t / ticks;
      const ease = 1 - Math.pow(1 - p, 3);
      setVal(target * ease);
      if (t >= ticks) clearInterval(timer);
    }, 34);
    return () => clearInterval(timer);
  }, [target]);
  return <>{decimals ? val.toFixed(decimals) : Math.round(val)}</>;
}

// ── Tool ticker ───────────────────────────────────────────────────────────────
const TOOLS = [
  { name: "Gitea",          color: "#38bdf8" },
  { name: "Jenkins",        color: "#f87171" },
  { name: "Harbor",         color: "#0ea5e9" },
  { name: "Kustomize",      color: "#4ade80" },
  { name: "ArgoCD",         color: "#c084fc" },
  { name: "CloudNativePG",  color: "#38bdf8" },
  { name: "Strimzi",        color: "#fbbf24" },
  { name: "RabbitMQ",       color: "#f87171" },
  { name: "Redis",          color: "#f87171" },
  { name: "MinIO",          color: "#fbbf24" },
  { name: "DefectDojo",     color: "#c084fc" },
  { name: "DependencyTrack",color: "#4ade80" },
  { name: "Vault",          color: "#fbbf24" },
  { name: "Kubernetes",     color: "#38bdf8" },
];

// ── Pipeline stages ───────────────────────────────────────────────────────────
const STAGES = [
  { num: "01", title: "Build", blurb: "Multi-stage image from the language profile, dependencies cached" },
  { num: "02", title: "Test",  blurb: "Unit and integration suites, JUnit results published" },
  { num: "03", title: "Scan",  blurb: "SBOM to DependencyTrack, findings to DefectDojo" },
  { num: "04", title: "Publish", blurb: "Signed image pushed to Harbor with a retention policy" },
  { num: "05", title: "Deliver", blurb: "Manifest bump commits, ArgoCD syncs the cluster" },
];

// ── Wizard preview steps ──────────────────────────────────────────────────────
const WIZARD_STEPS = [
  { label: "Identity",    chips: [{ v: "ledger-reconciler", on: true }, { v: "payments-core", on: true }, { v: "Payments", on: false }] },
  { label: "Build tool",  chips: [{ v: "maven", on: true }, { v: "gradle", on: false }, { v: "go", on: false }, { v: "node", on: false }, { v: "python", on: false }] },
  { label: "Runtime",     chips: [{ v: "8080", on: true }, { v: "/health/liveness", on: true }, { v: "/health/readiness", on: true }, { v: "medium · 500m / 1Gi", on: true }] },
  { label: "Infra",       chips: [{ v: "PostgreSQL", on: true }, { v: "Kafka", on: true }, { v: "Redis", on: false }, { v: "RabbitMQ", on: false }] },
  { label: "Deps",        chips: [{ v: "auth-gateway", on: true }, { v: "fx-rates", on: true }, { v: "schema-registry", on: false }] },
  { label: "Review",      chips: [{ v: "9 resources", on: true }, { v: "2 dependencies", on: true }, { v: "tier-2-prod", on: true }] },
  { label: "Provision",   chips: [{ v: "8/8 complete", on: true }, { v: "3.0s", on: true }, { v: "eu-prod-01", on: true }] },
];

// ── Capabilities cards ────────────────────────────────────────────────────────
const CAPS = [
  { name: "Source",     tool: "Gitea",         blurb: "Managed Git with branch protections and webhook events",   color: "#38bdf8" },
  { name: "CI",         tool: "Jenkins",        blurb: "Multibranch pipeline from the language template",          color: "#f87171" },
  { name: "Registry",   tool: "Harbor",         blurb: "Private image registry with CVE scanning and SBOM",        color: "#0ea5e9" },
  { name: "Manifests",  tool: "Kustomize",      blurb: "Base + three environment overlays, committed to GitOps",   color: "#4ade80" },
  { name: "Delivery",   tool: "ArgoCD",         blurb: "GitOps app with auto-sync and self-heal on main",          color: "#c084fc" },
  { name: "Databases",  tool: "CloudNativePG",  blurb: "Managed Postgres cluster with automatic failover",         color: "#38bdf8" },
  { name: "Messaging",  tool: "Strimzi + RabbitMQ", blurb: "Kafka topics and AMQP queues from a single checkbox", color: "#fbbf24" },
  { name: "Cache",      tool: "Redis · MinIO",  blurb: "In-memory cache and S3-compatible object storage",         color: "#f87171" },
  { name: "Security",   tool: "DefectDojo · DependencyTrack", blurb: "SBOM generation and finding triage on every build", color: "#c084fc" },
];

export function LandingPage() {
  const { theme, toggle: toggleTheme } = useTheme();
  const [wizStep, setWizStep] = useState(0);
  const [pinned, setPinned] = useState(false);
  const [pipLit, setPipLit] = useState(0);

  // Auto-advance wizard
  useEffect(() => {
    if (pinned) return;
    const t = setInterval(() => setWizStep(s => (s + 1) % WIZARD_STEPS.length), 2600);
    return () => clearInterval(t);
  }, [pinned]);

  // Pipeline dot animation
  useEffect(() => {
    const t = setInterval(() => setPipLit(n => (n + 1) % 5), 1200);
    return () => clearInterval(t);
  }, []);

  const doubled = [...TOOLS, ...TOOLS];

  return (
    <div style={{ minHeight: "100vh", background: "var(--bg)", color: "var(--text)" }}>
      {/* ── Header ── */}
      <header style={{ position: "sticky", top: 0, zIndex: 10, height: 60, borderBottom: "1px solid var(--line)", background: "var(--bg)", padding: "0 32px", display: "flex", alignItems: "center", maxWidth: 1180, margin: "0 auto", width: "100%" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ width: 26, height: 26, borderRadius: 7, background: "var(--accent)", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 13, fontWeight: 700, color: "var(--accent-ink)" }}>N</span>
          <span style={{ fontSize: 13, fontWeight: 600 }}>DevPortal</span>
        </div>
        <nav style={{ display: "flex", gap: 28, marginLeft: 40 }}>
          {["The run","What it creates","Onboarding","Control"].map(label => (
            <a key={label} href={`#${label.toLowerCase().replace(/ /g,"-")}`} style={{ fontSize: 13, fontWeight: 500, color: "var(--muted)", textDecoration: "none" }}
              onMouseEnter={e => (e.currentTarget as HTMLElement).style.color = "var(--text)"}
              onMouseLeave={e => (e.currentTarget as HTMLElement).style.color = "var(--muted)"}
            >{label}</a>
          ))}
        </nav>
        <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 12 }}>
          <button onClick={toggleTheme} className="btn btn-ghost btn-sm" style={{ width: 30, height: 30, padding: 0 }}>{theme === "dark" ? "☀" : "☾"}</button>
          <Link to="/signin" style={{ fontSize: 13, color: "var(--muted)", textDecoration: "none" }}>Sign in</Link>
          <Link to="/applications/new" className="btn btn-primary" style={{ textDecoration: "none", height: 30, fontSize: 12.5 }}>Open portal</Link>
        </div>
      </header>

      {/* ── Hero ── */}
      <section style={{ maxWidth: 1180, margin: "0 auto", padding: "80px 32px 56px" }}>
        <div style={{ display: "grid", gridTemplateColumns: "minmax(0,.92fr) minmax(0,1.08fr)", gap: 52, alignItems: "center" }}>
          {/* Left */}
          <div>
            <p style={{ fontSize: 11.5, color: "var(--faint)", letterSpacing: ".05em", marginBottom: 18, fontFamily: "JetBrains Mono,monospace" }}>one guided flow · nine systems · four minutes</p>
            <h1 style={{ fontSize: 50, fontWeight: 600, letterSpacing: "-0.04em", lineHeight: 1.04, marginBottom: 20, textWrap: "balance" } as React.CSSProperties}>Watch a service onboard itself.</h1>
            <p style={{ fontSize: 16, color: "var(--muted)", lineHeight: 1.6, maxWidth: "44ch", marginBottom: 32 }}>Every service provisioned from a single form — Git repo, Jenkins pipeline, Harbor registry, Kubernetes manifests and ArgoCD, wired together.</p>
            <div style={{ display: "flex", gap: 12, flexWrap: "wrap", marginBottom: 40 }}>
              <Link to="/applications/new" className="btn btn-primary btn-lg" style={{ textDecoration: "none" }}>Onboard a service</Link>
              <a href="#the-run" className="btn btn-secondary btn-lg" style={{ textDecoration: "none" }}>Watch a real run</a>
            </div>
            <div style={{ borderTop: "1px solid var(--line)", paddingTop: 24, display: "flex", gap: 34 }}>
              {[{v:148,label:"services onboarded"},{v:21,label:"applications"},{v:6,label:"clusters"},{v:96.4,label:"pipeline success",pct:true}].map(s => (
                <div key={s.label}>
                  <div style={{ fontSize: 23, fontWeight: 600, letterSpacing: "-0.03em", fontVariantNumeric: "tabular-nums", color: s.pct ? "var(--ok)" : "var(--text)" }}>
                    <CountUp target={s.v} decimals={s.pct ? 1 : 0} />{s.pct ? "%" : ""}
                  </div>
                  <div style={{ fontSize: 12, color: "var(--faint)", marginTop: 2 }}>{s.label}</div>
                </div>
              ))}
            </div>
          </div>
          {/* Right — terminal */}
          <HeroTerminal />
        </div>
      </section>

      {/* ── Tool ticker ── */}
      <div style={{ borderTop: "1px solid var(--line)", borderBottom: "1px solid var(--line)", background: "var(--panel)", padding: "14px 0", overflow: "hidden" }}>
        <div className="landing-ticker" style={{ display: "flex", width: "max-content", gap: 36 }}>
          {doubled.map((t, i) => (
            <div key={i} style={{ display: "flex", alignItems: "center", gap: 9, flexShrink: 0 }}>
              <span style={{ width: 6, height: 6, borderRadius: 2, background: t.color, flexShrink: 0 }} />
              <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12.5, color: "var(--muted)", whiteSpace: "nowrap" }}>{t.name}</span>
            </div>
          ))}
        </div>
      </div>

      {/* ── Pipeline section ── */}
      <section id="the-run" style={{ maxWidth: 1180, margin: "0 auto", padding: "72px 32px 8px" }}>
        <p className="overline accent" style={{ marginBottom: 10 }}>The pipeline it builds for you</p>
        <h2 style={{ fontSize: 32, fontWeight: 600, letterSpacing: "-0.03em", marginBottom: 14 }}>Commit to production, without a single hand-off.</h2>
        <p style={{ fontSize: 14.5, color: "var(--muted)", maxWidth: "58ch", marginBottom: 32, lineHeight: 1.6 }}>One provisioning run creates the full CI/CD chain. Every stage is wired in — you just push.</p>
        <div style={{ background: "var(--panel)", borderRadius: 12, padding: "36px 28px 30px" }}>
          {/* Dot track */}
          <div style={{ position: "relative", height: 8, margin: "0 28px 26px" }}>
            <div style={{ position: "absolute", top: 3, left: 0, right: 0, height: 2, background: "var(--line)" }} />
            <div className="landing-pip-dot" style={{ position: "absolute", top: 0, width: 8, height: 8, borderRadius: "50%", background: "var(--accent)", boxShadow: "0 0 12px var(--accent)" }} />
          </div>
          {/* Stage cards */}
          <div style={{ display: "grid", gridTemplateColumns: "repeat(5,1fr)", gap: 14 }}>
            {STAGES.map((s, i) => {
              const lit = pipLit === i;
              return (
                <div key={i} className={`pipe-card${lit ? " lit" : ""}`}>
                  <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, fontWeight: 600, letterSpacing: ".04em", marginBottom: 6 }}>{s.num}</div>
                  <div style={{ fontSize: 13, fontWeight: 600, color: "var(--text)", marginBottom: 6 }}>{s.title}</div>
                  <div style={{ fontSize: 12, color: "var(--muted)", lineHeight: 1.5 }}>{s.blurb}</div>
                </div>
              );
            })}
          </div>
        </div>
      </section>

      {/* ── Wizard preview ── */}
      <section id="onboarding" style={{ maxWidth: 1180, margin: "0 auto", padding: "72px 32px" }}>
        <div style={{ display: "grid", gridTemplateColumns: "minmax(0,.85fr) minmax(0,1.15fr)", gap: 52, alignItems: "start" }}>
          {/* Left */}
          <div>
            <p className="overline accent" style={{ marginBottom: 10 }}>The wizard</p>
            <h2 style={{ fontSize: 32, fontWeight: 600, letterSpacing: "-0.03em", marginBottom: 14 }}>Seven questions. No YAML.</h2>
            <p style={{ fontSize: 14.5, color: "var(--muted)", lineHeight: 1.6, marginBottom: 20 }}>Answer a handful of fields and the platform provisions nine systems on your behalf. No tickets, no waiting.</p>
            {WIZARD_STEPS.map((ws, i) => (
              <div key={i} onClick={() => { setWizStep(i); setPinned(true); }}
                style={{ display: "flex", gap: 12, padding: "8px 11px", borderRadius: 8, cursor: "pointer", transition: "background .2s ease", background: wizStep === i ? "var(--panel)" : "transparent", color: wizStep === i ? "var(--text)" : "var(--muted)", marginBottom: 2 }}>
                <span style={{ width: 20, height: 20, borderRadius: 6, flexShrink: 0, display: "flex", alignItems: "center", justifyContent: "center", fontSize: 10.5, fontWeight: 600, fontFamily: "JetBrains Mono,monospace", background: wizStep === i ? "var(--accent)" : "var(--card)", color: wizStep === i ? "var(--accent-ink)" : "var(--faint)" }}>{i + 1}</span>
                <span style={{ fontSize: 13.5, fontWeight: 500 }}>{ws.label}</span>
              </div>
            ))}
            <Link to="/applications/new" style={{ display: "inline-block", marginTop: 16, fontSize: 12, color: "var(--accent)", textDecoration: "none" }}>Walk through it yourself →</Link>
          </div>
          {/* Right — preview panel */}
          <div style={{ background: "var(--panel)", border: "1px solid var(--line)", borderRadius: 12, overflow: "hidden" }}>
            {/* Pip bar header */}
            <div style={{ padding: "14px 18px", borderBottom: "1px solid var(--line)", display: "flex", alignItems: "center", gap: 8 }}>
              {WIZARD_STEPS.map((_, i) => (
                <div key={i} style={{ flex: 1, height: 3, borderRadius: 2, background: i < wizStep ? "var(--line2)" : i === wizStep ? "var(--accent)" : "var(--line)", transition: "background .3s ease" }} />
              ))}
              <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, color: "var(--faint)", marginLeft: 8, flexShrink: 0 }}>step {wizStep + 1} / 7</span>
            </div>
            {/* Body */}
            <div style={{ minHeight: 308, padding: "26px 24px 30px" }}>
              <p className="overline accent" style={{ marginBottom: 8 }}>{WIZARD_STEPS[wizStep].label}</p>
              <h3 style={{ fontSize: 21, fontWeight: 600, letterSpacing: "-0.025em", marginBottom: 10 }}>{WIZARD_STEPS[wizStep].label}</h3>
              <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 16 }}>
                {WIZARD_STEPS[wizStep].chips.map(chip => (
                  <span key={chip.v} style={{
                    height: 26, padding: "0 11px", borderRadius: 7, fontSize: 12, fontWeight: 500, fontFamily: "JetBrains Mono,monospace",
                    background: chip.on ? "var(--accent-soft)" : "var(--card)",
                    border: `1px solid ${chip.on ? "var(--accent)" : "var(--line2)"}`,
                    color: chip.on ? "var(--accent)" : "var(--muted)",
                    display: "inline-flex", alignItems: "center",
                  }}>{chip.v}</span>
                ))}
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ── Capabilities ── */}
      <div id="what-it-creates" style={{ background: "var(--panel)", borderTop: "1px solid var(--line)", borderBottom: "1px solid var(--line)" }}>
        <section style={{ maxWidth: 1180, margin: "0 auto", padding: "72px 32px" }}>
          <p className="overline accent" style={{ marginBottom: 10 }}>One flow, nine systems</p>
          <h2 style={{ fontSize: 32, fontWeight: 600, letterSpacing: "-0.03em", marginBottom: 10 }}>Everything a service needs, wired up before the first commit.</h2>
          <p style={{ fontSize: 14.5, color: "var(--muted)", marginBottom: 36, lineHeight: 1.6 }}>Each of these used to be a ticket to a different team.</p>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(230px,1fr))", gap: 12 }}>
            {CAPS.map(c => (
              <div key={c.name} style={{ background: "var(--bg)", border: "1px solid var(--line)", borderRadius: 10, padding: 18, transition: "border-color .2s ease", cursor: "default" }}
                onMouseEnter={e => (e.currentTarget as HTMLElement).style.borderColor = "var(--accent)"}
                onMouseLeave={e => (e.currentTarget as HTMLElement).style.borderColor = "var(--line)"}
              >
                <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 6 }}>
                  <span style={{ width: 8, height: 8, borderRadius: 2, background: c.color, flexShrink: 0 }} />
                  <span style={{ fontSize: 13.5, fontWeight: 600 }}>{c.name}</span>
                </div>
                <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, color: "var(--accent)", marginBottom: 8 }}>{c.tool}</div>
                <div style={{ fontSize: 12.5, color: "var(--muted)", lineHeight: 1.5 }}>{c.blurb}</div>
              </div>
            ))}
          </div>
        </section>
      </div>

      {/* ── Control section ── */}
      <section id="control" style={{ maxWidth: 1180, margin: "0 auto", padding: "72px 32px" }}>
        <p className="overline accent" style={{ marginBottom: 10 }}>For the platform team</p>
        <h2 style={{ fontSize: 32, fontWeight: 600, letterSpacing: "-0.03em", marginBottom: 32 }}>Defaults you set once, applied everywhere.</h2>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(260px,1fr))", gap: 14 }}>
          {[
            { title: "Environment profiles",  blurb: "Define CPU, memory and replica bounds per tier. Services inherit them automatically.", cta: "Manage profiles →", to: "/platform" },
            { title: "Language profiles",      blurb: "Set probe timings and base images per language. One change propagates to all services.", cta: "Edit profiles →",  to: "/platform" },
            { title: "Pipeline templates",     blurb: "Jenkinsfile and Dockerfile templates rendered with service context at provisioning time.", cta: "Open templates →", to: "/templates" },
            { title: "Platform credentials",  blurb: "Store integration tokens centrally. Services reference them by name, never by value.", cta: "View credentials →", to: "/credentials" },
          ].map(card => (
            <div key={card.title} style={{ background: "var(--panel)", border: "1px solid var(--line)", borderRadius: 10, padding: 18 }}>
              <div style={{ fontSize: 14.5, fontWeight: 600, marginBottom: 8 }}>{card.title}</div>
              <div style={{ fontSize: 12.5, color: "var(--muted)", lineHeight: 1.6, marginBottom: 14 }}>{card.blurb}</div>
              <Link to={card.to} style={{ fontSize: 12, color: "var(--accent)", textDecoration: "none" }}>{card.cta}</Link>
            </div>
          ))}
        </div>
      </section>

      {/* ── CTA ── */}
      <section style={{ maxWidth: 1180, margin: "0 auto", padding: "0 32px 72px" }}>
        <div style={{ background: "var(--accent-soft)", border: "1px solid var(--accent)", borderRadius: 14, padding: "44px 40px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 32, flexWrap: "wrap" }}>
          <h2 style={{ fontSize: 26, fontWeight: 600, margin: 0, maxWidth: "32ch" }}>Ready to onboard your first service in four minutes?</h2>
          <div style={{ display: "flex", gap: 12, flexShrink: 0 }}>
            <Link to="/applications/new" className="btn btn-primary btn-lg" style={{ textDecoration: "none" }}>Get started</Link>
            <Link to="/templates" className="btn btn-secondary btn-lg" style={{ textDecoration: "none" }}>Read the templates</Link>
          </div>
        </div>
      </section>

      {/* ── Footer ── */}
      <footer style={{ borderTop: "1px solid var(--line)", padding: 32, maxWidth: 1180, margin: "0 auto", display: "flex", alignItems: "center", justifyContent: "space-between", flexWrap: "wrap", gap: 16 }}>
        <div style={{ fontSize: 12.5, color: "var(--faint)" }}>NexBridge Technologies · internal platform</div>
        <div style={{ display: "flex", gap: 20 }}>
          {["Status","Docs","API","Support","Changelog"].map(l => (
            <a key={l} href="#" style={{ fontSize: 12.5, color: "var(--faint)", textDecoration: "none" }}
              onMouseEnter={e => (e.currentTarget as HTMLElement).style.color = "var(--text)"}
              onMouseLeave={e => (e.currentTarget as HTMLElement).style.color = "var(--faint)"}
            >{l}</a>
          ))}
        </div>
      </footer>
    </div>
  );
}
