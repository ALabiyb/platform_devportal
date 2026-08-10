// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useNavigate } from "react-router-dom";
import { useSetupStatus } from "@/lib/api";

// ── Flow diagram ──────────────────────────────────────────────────────────────

const FLOW_NODES = [
  {
    label: "GitLab repo",
    delay: "0s",
    icon: (
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" width="18" height="18">
        <path d="M8 13.4 1.6 8.8l1-3.1 1.5 4.6h7.8l1.5-4.6 1 3.1L8 13.4Z" />
        <path d="M4.1 5.7 2.6 10.3M11.9 5.7l1.5 4.6M4.1 5.7 8 2.6l3.9 3.1" />
      </svg>
    ),
  },
  {
    label: "Jenkins pipeline",
    delay: "1s",
    icon: (
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" width="18" height="18">
        <circle cx="3" cy="3" r="1.6" /><circle cx="3" cy="13" r="1.6" /><circle cx="13" cy="8" r="1.6" />
        <path d="M3 4.6V11.4M4.4 3.6l7.2 3.6M4.4 12.4l7.2-3.6" />
      </svg>
    ),
  },
  {
    label: "Harbor registry",
    delay: "2s",
    icon: (
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" width="18" height="18">
        <rect x="2" y="2.5" width="12" height="3.4" rx="1" />
        <rect x="2" y="6.8" width="12" height="3.4" rx="1" />
        <rect x="2" y="11.1" width="12" height="2.4" rx="1" />
      </svg>
    ),
  },
  {
    label: "DefectDojo",
    delay: "3s",
    icon: (
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" width="18" height="18">
        <path d="M8 1.6 13.5 3.4V7.6c0 3.4-2.3 5.7-5.5 6.8-3.2-1.1-5.5-3.4-5.5-6.8V3.4L8 1.6Z" />
        <path d="M5.5 8l1.7 1.7L10.5 6.2" />
      </svg>
    ),
  },
  {
    label: "ArgoCD apps",
    delay: "4s",
    icon: (
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" width="18" height="18">
        <path d="M8 2v3M8 11v3M2 8h3M11 8h3" /><circle cx="8" cy="8" r="2.4" />
      </svg>
    ),
  },
  {
    label: "K8s + DB",
    delay: "5s",
    icon: (
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" width="18" height="18">
        <path d="M8 1.6 13.5 5v6L8 14.4 2.5 11V5L8 1.6Z" />
        <ellipse cx="8" cy="5.5" rx="2.5" ry="1" />
        <path d="M5.5 5.5v3c0 .55 1.12 1 2.5 1s2.5-.45 2.5-1v-3" />
      </svg>
    ),
  },
];

// 18s cycle, 2s spacing → lines appear sequentially, all visible near the end
const FLOW_LOG_LINES = [
  { text: "✓ created gitlab repo", delay: "0s" },
  { text: "✓ configured jenkins pipeline", delay: "2s" },
  { text: "✓ created harbor project", delay: "4s" },
  { text: "✓ created defectdojo product", delay: "6s" },
  { text: "✓ created argocd apps (dev/uat/prod)", delay: "8s" },
  { text: "✓ provisioned namespaces + databases", delay: "10s" },
  { text: "✓ done in 41s", delay: "12s" },
];

const ROLES = [
  {
    role: "Developer",
    desc: "Create fully-configured project environments without waiting on DevOps. Your repo, pipeline, registry, and Kubernetes namespaces are ready in minutes.",
  },
  {
    role: "DevOps Lead",
    desc: "Enforce team standards automatically. Every project inherits the correct pipeline template, security gates, and namespace conventions.",
  },
  {
    role: "Admin",
    desc: "Manage users, teams, and credentials from one place. Audit every significant action in real time with an immutable event log.",
  },
];

// ── Component ──────────────────────────────────────────────────────────────────

export function LandingPage() {
  const navigate = useNavigate();
  const { data: setup, isLoading: setupLoading } = useSetupStatus();
  const needsSetup = setup?.needs_setup ?? false;

  const primary = "hsl(var(--primary))";
  const primaryGlow = "hsl(var(--brand-hue) 89% 48% / 0.2)";

  return (
    <div style={{ minWidth: 1024, background: "#0f172a", color: "#f8fafc", minHeight: "100vh", fontFamily: "'IBM Plex Sans', system-ui, sans-serif", WebkitFontSmoothing: "antialiased" }}>
      <div style={{ maxWidth: 1120, margin: "0 auto", padding: "0 40px" }}>

        {/* ── Nav ── */}
        <header style={{ display: "flex", alignItems: "center", justifyContent: "space-between", height: 72 }}>
          <span style={{ fontSize: 18, fontWeight: 700, letterSpacing: "-0.02em", color: primary }}>DevPortal</span>
          <nav style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <button
              onClick={() => navigate("/login")}
              className="landing-signin-btn"
            >
              Sign in
            </button>
            {!setupLoading && needsSetup && (
              <button
                onClick={() => navigate("/register")}
                style={{
                  height: 36, borderRadius: 6, border: "none",
                  background: primary, color: "#fff",
                  fontSize: 13, fontWeight: 500, padding: "0 16px", cursor: "pointer",
                }}
              >
                Get started
              </button>
            )}
          </nav>
        </header>

        {/* ── Hero ── */}
        <section style={{ padding: "88px 0 64px", maxWidth: 680 }}>
          <div style={{ fontSize: 12, fontWeight: 600, letterSpacing: "0.06em", color: "#94a3b8", textTransform: "uppercase", marginBottom: 16 }}>
            Internal platform · NexBridge Technologies
          </div>
          <h1 style={{ fontSize: 44, lineHeight: 1.15, fontWeight: 700, letterSpacing: "-0.02em", margin: "0 0 20px", color: "#f8fafc" }}>
            Ship a fully-configured project environment in one click.
          </h1>
          <p style={{ fontSize: 16, lineHeight: 1.6, color: "#94a3b8", margin: "0 0 32px" }}>
            DevPortal provisions your repo, pipeline, registry, security scanning and Kubernetes environments automatically — no DevOps toolchain knowledge required.
          </p>
          <div style={{ display: "flex", gap: 12 }}>
            {!setupLoading && needsSetup ? (
              <>
                <button
                  onClick={() => navigate("/register")}
                  style={{ height: 44, borderRadius: 6, border: "none", background: primary, color: "#fff", fontSize: 14, fontWeight: 600, padding: "0 22px", cursor: "pointer" }}
                >
                  Create admin account
                </button>
                <button
                  onClick={() => navigate("/login")}
                  style={{ height: 44, borderRadius: 6, border: "1px solid #334155", background: "transparent", color: "#f8fafc", fontSize: 14, fontWeight: 500, padding: "0 20px", cursor: "pointer" }}
                >
                  Sign in
                </button>
              </>
            ) : (
              <button
                onClick={() => navigate("/login")}
                style={{ height: 44, borderRadius: 6, border: "none", background: primary, color: "#fff", fontSize: 14, fontWeight: 600, padding: "0 22px", cursor: "pointer" }}
              >
                Sign in to DevPortal
              </button>
            )}
          </div>
          {!setupLoading && needsSetup && (
            <p style={{ fontSize: 12, color: "#64748b", marginTop: 12 }}>
              First time here? Create the one-time admin account to get started. Registration is disabled once an admin exists.
            </p>
          )}
        </section>

        {/* ── Animated provisioning flow ── */}
        <section style={{ padding: "8px 0 64px", borderTop: "1px solid #334155" }}>
          <h2 style={{ fontSize: 12, fontWeight: 600, letterSpacing: "0.06em", color: "#94a3b8", textTransform: "uppercase", margin: "40px 0 24px" }}>
            What happens when you submit a project
          </h2>

          <div
            style={{
              "--flow-primary": primary,
              "--flow-primary-glow": primaryGlow,
              border: "1px solid #334155",
              background: "#0b1220",
              borderRadius: 14,
              padding: "36px 32px 28px",
              overflow: "hidden",
            } as React.CSSProperties}
          >
            {/* Node row */}
            <div style={{ position: "relative", height: 88, display: "flex", alignItems: "flex-start", justifyContent: "space-between" }}>
              {/* Connecting rail */}
              <div style={{ position: "absolute", top: 22, left: 22, right: 22, height: 2, background: "#1e2b3f", borderRadius: 2 }} />
              {/* Traveling dot */}
              <div
                style={{
                  position: "absolute",
                  top: 21,
                  left: 22,
                  width: 5,
                  height: 5,
                  borderRadius: "50%",
                  background: primary,
                  boxShadow: `0 0 10px 2px ${primaryGlow}`,
                  animation: "flowTravel 6s linear infinite",
                }}
              />
              {FLOW_NODES.map((node) => (
                <div key={node.label} style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 8, width: 64, position: "relative", zIndex: 1 }}>
                  <div
                    style={{
                      width: 44,
                      height: 44,
                      borderRadius: "50%",
                      background: "#0f172a",
                      border: "1.5px solid #334155",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      color: "#64748b",
                      animation: `flowNodeGlow 6s ease-in-out infinite`,
                      animationDelay: node.delay,
                    }}
                  >
                    {node.icon}
                  </div>
                  <span style={{ fontSize: 11, color: "#94a3b8", textAlign: "center", lineHeight: 1.3 }}>{node.label}</span>
                </div>
              ))}
            </div>

            {/* Terminal log */}
            <div
              style={{
                marginTop: 28,
                border: "1px solid #1e2b3f",
                background: "#070d18",
                borderRadius: 8,
                padding: "14px 16px",
                fontFamily: "'IBM Plex Mono', monospace",
                fontSize: 12,
                height: 220,
                overflow: "hidden",
              }}
            >
              {FLOW_LOG_LINES.map((line) => (
                <div
                  key={line.text}
                  style={{
                    opacity: 0,
                    color: "#93c5fd",
                    padding: "3px 0",
                    animation: `flowLogLine 18s linear infinite`,
                    animationDelay: line.delay,
                  }}
                >
                  {line.text}
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* ── Built for every role ── */}
        <section style={{ padding: "8px 0 80px", borderTop: "1px solid #334155" }}>
          <h2 style={{ fontSize: 12, fontWeight: 600, letterSpacing: "0.06em", color: "#94a3b8", textTransform: "uppercase", margin: "40px 0 20px" }}>
            Built for every role
          </h2>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 16 }}>
            {ROLES.map((r) => (
              <div key={r.role} style={{ border: "1px solid #334155", background: "#1e293b", borderRadius: 10, padding: 20 }}>
                <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 8, color: "#f8fafc" }}>{r.role}</div>
                <div style={{ fontSize: 13, color: "#94a3b8", lineHeight: 1.6 }}>{r.desc}</div>
              </div>
            ))}
          </div>
        </section>

        {/* ── Footer ── */}
        <footer style={{ borderTop: "1px solid #334155", padding: "24px 0", fontSize: 12, color: "#64748b" }}>
          © 2026 NexBridge Technologies · Internal use only
        </footer>
      </div>
    </div>
  );
}
