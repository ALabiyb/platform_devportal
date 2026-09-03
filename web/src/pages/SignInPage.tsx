// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState, useEffect, useRef } from "react";
import { useNavigate, Link } from "react-router-dom";
import { useLocalLogin } from "@/lib/api";
import { ApiError } from "@/lib/queryClient";
import { useTheme } from "@/components/Layout";

// ── Hero terminal (mini version for sign-in left panel) ───────────────────
const LOG_LINES = [
  { mark: "·", text: "resolving language profile", note: "maven · java 21", markColor: "var(--faint)" },
  { mark: "✓", text: "git repository created", dur: "0.6s", markColor: "var(--ok)" },
  { mark: "✓", text: "jenkins pipeline registered", dur: "0.9s", markColor: "var(--ok)" },
  { mark: "✓", text: "harbor project + robot account", dur: "1.2s", markColor: "var(--ok)" },
  { mark: "✓", text: "kustomize manifests generated", dur: "1.5s", markColor: "var(--ok)" },
  { mark: "✓", text: "argocd application synced", dur: "1.8s", markColor: "var(--ok)" },
  { mark: "→", text: "service ready on eu-prod-01", markColor: "var(--accent)" },
];
const COMMAND = "devportal provision ledger-reconciler --app payments-core";
const FRAME_MS = 44;
const FRAMES_PER_LINE = 12;
const HOLD_FRAMES = 42;
const TOTAL = COMMAND.length + LOG_LINES.length * FRAMES_PER_LINE + HOLD_FRAMES;

function MiniTerminal() {
  const [frame, setFrame] = useState(0);
  const [elapsed, setElapsed] = useState(0);
  const startRef = useRef(Date.now());

  useEffect(() => {
    const id = setInterval(() => {
      setFrame(f => (f + 1) % TOTAL);
      setElapsed(Math.floor((Date.now() - startRef.current) / 1000));
    }, FRAME_MS);
    return () => clearInterval(id);
  }, []);

  const cmdChars  = Math.min(frame, COMMAND.length);
  const afterCmd  = frame - COMMAND.length;
  const linesShown = afterCmd < 0 ? 0 : Math.floor(afterCmd / FRAMES_PER_LINE);

  return (
    <div style={{
      background: "var(--panel)", border: "1px solid var(--line)",
      borderRadius: 10, overflow: "hidden", maxWidth: 460,
      boxShadow: "0 24px 48px -24px rgba(0,0,0,.5)",
    }}>
      {/* Title bar */}
      <div style={{
        display: "flex", alignItems: "center", gap: 8,
        padding: "9px 12px", background: "var(--card)",
        borderBottom: "1px solid var(--line)",
      }}>
        <div style={{ display: "flex", gap: 5 }}>
          {["var(--bad)","var(--warn)","var(--ok)"].map((c,i) => (
            <div key={i} style={{ width: 8, height: 8, borderRadius: "50%", background: c }} />
          ))}
        </div>
        <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, color: "var(--faint)", flex: 1 }}>
          devportal — provisioning
        </span>
        <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, color: "var(--faint)" }}>
          {elapsed}s
        </span>
      </div>
      {/* Body */}
      <div style={{ padding: "14px 16px 18px", minHeight: 186, fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, lineHeight: "19px" }}>
        {/* Command */}
        <div style={{ marginBottom: 10 }}>
          <span style={{ color: "var(--accent)" }}>$ </span>
          <span style={{ color: "var(--text)" }}>{COMMAND.slice(0, cmdChars)}</span>
          {cmdChars < COMMAND.length && (
            <span style={{ display: "inline-block", width: 7, height: 14, background: "var(--accent)", animation: "dcblink 1s step-end infinite", verticalAlign: "middle" }} />
          )}
        </div>
        {/* Log lines */}
        {LOG_LINES.slice(0, linesShown).map((line, i) => (
          <div key={i} style={{ display: "grid", gridTemplateColumns: "14px 1fr auto", gap: 8, animation: "dcrise .22s ease-out both", marginBottom: 1 }}>
            <span style={{ color: line.markColor }}>{line.mark}</span>
            <span style={{ color: "var(--muted)" }}>{line.text}</span>
            <span style={{ color: "var(--faint)", fontSize: 10.5 }}>{line.note || line.dur || ""}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Theme toggle button ───────────────────────────────────────────────────
function ThemeBtn() {
  const { theme, toggle } = useTheme();
  return (
    <button onClick={toggle} style={{
      height: 30, padding: "0 12px", borderRadius: 7,
      border: "1px solid var(--line)", background: "transparent",
      color: "var(--faint)", fontSize: 12, cursor: "pointer",
      display: "flex", alignItems: "center", gap: 5,
    }}>
      {theme === "dark" ? "☀ Light" : "☽ Dark"}
    </button>
  );
}

// ── Sign In page ──────────────────────────────────────────────────────────
export function SignInPage() {
  const navigate = useNavigate();
  const login = useLocalLogin();

  const [email, setEmail]         = useState("");
  const [password, setPassword]   = useState("");
  const [reveal, setReveal]       = useState(false);
  const [remember, setRemember]   = useState(true);
  const [emailError, setEmailError]   = useState("");
  const [passError, setPassError]     = useState("");
  const [formError, setFormError]     = useState("");
  const [ssoState, setSsoState]       = useState<"idle" | "loading">("idle");
  const [submitState, setSubmitState] = useState<"idle" | "checking">("idle");

  // Email validation — accept any valid email address
  const validateEmail = (v: string) => /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(v);

  function handleEmailBlur() {
    if (email && !validateEmail(email))
      setEmailError("Enter a valid email address.");
    else setEmailError("");
  }

  async function handleSso() {
    setSsoState("loading");
    setTimeout(() => {
      setSsoState("idle");
      // SSO would redirect; here we navigate to dashboard
      navigate("/");
    }, 1300);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setFormError(""); setPassError("");
    if (!email) { setEmailError("Enter your work email."); return; }
    if (!validateEmail(email)) { setEmailError("Enter a valid email address."); return; }
    if (password.length < 8) {
      setPassError("Password must be at least 8 characters. Three failed attempts locks the account for 15 minutes.");
      return;
    }
    setSubmitState("checking");
    try {
      await login.mutateAsync({ email, password });
      navigate("/");
    } catch (err) {
      setSubmitState("idle");
      if (err instanceof ApiError && err.status === 401) {
        setFormError("Invalid email or password. Please check your credentials and try again.");
      } else {
        setFormError("Something went wrong. Please try again.");
      }
    }
  }

  return (
    <div style={{
      display: "grid", gridTemplateColumns: "minmax(0,1fr) minmax(0,1.05fr)",
      minHeight: "100vh", background: "var(--bg)",
    }}>
      {/* Left panel */}
      <div style={{
        background: "var(--panel)", borderRight: "1px solid var(--line)",
        padding: "56px 8vw", display: "flex", flexDirection: "column",
        justifyContent: "center", gap: 32,
      }}>
        {/* Brand */}
        <Link to="/" style={{ textDecoration: "none", display: "flex", alignItems: "center", gap: 10, marginBottom: 32 }}>
          <div style={{
            width: 34, height: 34, borderRadius: 8, background: "var(--accent)",
            color: "var(--accent-ink)", display: "flex", alignItems: "center",
            justifyContent: "center", fontSize: 16, fontWeight: 700,
          }}>N</div>
          <div>
            <div style={{ fontSize: 14, fontWeight: 600, color: "var(--text)" }}>DevPortal</div>
            <div style={{ fontSize: 10, color: "var(--faint)", letterSpacing: ".04em" }}>NEXBRIDGE</div>
          </div>
        </Link>

        <h1 style={{
          fontSize: 34, fontWeight: 600, letterSpacing: "-0.035em",
          color: "var(--text)", margin: 0, maxWidth: "20ch", lineHeight: 1.15,
        }}>
          A new service in four minutes, not four weeks.
        </h1>
        <p style={{ fontSize: 14.5, color: "var(--muted)", margin: 0, lineHeight: 1.6 }}>
          Every service provisioned from a single form — Git repo, Jenkins pipeline, Harbor registry, Kubernetes manifests and ArgoCD, wired together.
        </p>

        <MiniTerminal />

        {/* Stats */}
        <div style={{ display: "flex", gap: 32, paddingTop: 20, borderTop: "1px solid var(--line)" }}>
          {[
            { val: "148", label: "services" },
            { val: "21",  label: "applications" },
            { val: "6",   label: "clusters" },
            { val: "96.4%", label: "pipeline success", accent: true },
          ].map(s => (
            <div key={s.label}>
              <div style={{
                fontSize: 19, fontWeight: 600, letterSpacing: "-0.03em",
                color: s.accent ? "var(--ok)" : "var(--text)",
                fontVariantNumeric: "tabular-nums",
              }}>{s.val}</div>
              <div style={{ fontSize: 12, color: "var(--faint)" }}>{s.label}</div>
            </div>
          ))}
        </div>
      </div>

      {/* Right panel */}
      <div style={{ padding: "24px 6vw 56px", display: "flex", flexDirection: "column" }}>
        {/* Top row */}
        <div style={{ display: "flex", justifyContent: "flex-end", alignItems: "center", gap: 12, height: 36, marginBottom: "auto" }}>
          <ThemeBtn />
          <a href="#" style={{ fontSize: 13, color: "var(--muted)", textDecoration: "none" }}>Need help?</a>
        </div>

        {/* Form */}
        <div style={{ maxWidth: 400, margin: "auto 0", width: "100%", display: "flex", flexDirection: "column", gap: 20 }}>
          <div>
            <h2 style={{ fontSize: 25, fontWeight: 600, letterSpacing: "-0.03em", margin: "0 0 6px", color: "var(--text)" }}>
              Sign in to DevPortal
            </h2>
            <p style={{ fontSize: 13.5, color: "var(--muted)", margin: 0 }}>
              Use SSO, or a break-glass account if SSO is unavailable.
            </p>
          </div>

          {/* SSO */}
          <button
            onClick={handleSso}
            disabled={ssoState === "loading"}
            style={{
              width: "100%", height: 44, borderRadius: 10,
              background: "var(--accent)", border: "1px solid var(--accent)",
              color: "var(--accent-ink)", fontSize: 14, fontWeight: 600,
              display: "flex", alignItems: "center", justifyContent: "center",
              gap: 10, cursor: "pointer",
              opacity: ssoState === "loading" ? .8 : 1,
              transition: "filter .12s",
            }}
          >
            {ssoState === "loading" ? (
              <>
                <span className="spinner" style={{ borderColor: "var(--accent-ink)", borderTopColor: "transparent" }} />
                Redirecting…
              </>
            ) : (
              <>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="3" y="11" width="18" height="11" rx="2" />
                  <path d="M7 11V7a5 5 0 0 1 10 0v4" />
                </svg>
                Continue with SSO
              </>
            )}
          </button>

          {/* Divider */}
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <div style={{ flex: 1, height: 1, background: "var(--line)" }} />
            <span style={{ fontSize: 11, fontWeight: 600, letterSpacing: ".06em", textTransform: "uppercase", color: "var(--faint)" }}>
              or use a break-glass account
            </span>
            <div style={{ flex: 1, height: 1, background: "var(--line)" }} />
          </div>

          <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: 14 }}>
            {/* Email */}
            <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <label style={{ fontSize: 12.5, fontWeight: 600, color: "var(--text)" }}>Work email</label>
              <input
                type="email" value={email}
                onChange={e => { setEmail(e.target.value); setEmailError(""); }}
                onBlur={handleEmailBlur}
                placeholder="you@nexbridge.io"
                className={`field field-lg${emailError ? " error" : ""}`}
              />
              {emailError && <p style={{ margin: 0, fontSize: 11.5, color: "var(--bad)" }}>{emailError}</p>}
            </div>

            {/* Password */}
            <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                <label style={{ fontSize: 12.5, fontWeight: 600, color: "var(--text)" }}>Password</label>
                <a href="#" style={{ fontSize: 12, color: "var(--accent)", textDecoration: "none" }}>Forgot password?</a>
              </div>
              <div style={{ display: "flex", gap: 8 }}>
                <input
                  type={reveal ? "text" : "password"} value={password}
                  onChange={e => { setPassword(e.target.value); setPassError(""); setFormError(""); }}
                  placeholder="••••••••"
                  className={`field field-lg${passError ? " error" : ""}`}
                  style={{ flex: 1 }}
                />
                <button
                  type="button" onClick={() => setReveal(r => !r)}
                  className="btn btn-secondary"
                  style={{ height: 40, borderRadius: 9, flexShrink: 0, padding: "0 12px" }}
                >
                  {reveal ? "Hide" : "Show"}
                </button>
              </div>
              {passError && <p style={{ margin: 0, fontSize: 11.5, color: "var(--bad)" }}>{passError}</p>}
            </div>

            {/* Remember */}
            <label style={{ display: "flex", alignItems: "center", gap: 9, cursor: "pointer" }}>
              <input
                type="checkbox" checked={remember}
                onChange={e => setRemember(e.target.checked)}
                style={{ display: "none" }}
              />
              <div style={{
                width: 17, height: 17, borderRadius: 5, flexShrink: 0,
                background: remember ? "var(--accent)" : "transparent",
                border: remember ? "none" : "1.5px solid var(--line2)",
                display: "flex", alignItems: "center", justifyContent: "center",
                color: "var(--accent-ink)", fontSize: 11, fontWeight: 700,
                transition: "background .12s, border .12s",
              }}>
                {remember ? "✓" : ""}
              </div>
              <span style={{ fontSize: 12.5, color: "var(--muted)" }}>Keep me signed in for 8 hours</span>
            </label>

            {/* Form error */}
            {formError && (
              <div style={{
                background: "var(--bad-soft)", border: "1px solid var(--bad)",
                borderRadius: 9, padding: "12px 14px",
                borderLeft: "4px solid var(--bad)",
              }}>
                <p style={{ margin: "0 0 3px", fontSize: 12.5, fontWeight: 600, color: "var(--bad)" }}>Sign-in blocked</p>
                <p style={{ margin: 0, fontSize: 12, color: "var(--muted)" }}>{formError}</p>
              </div>
            )}

            {/* Submit */}
            <button
              type="submit"
              disabled={submitState === "checking"}
              className="btn btn-secondary btn-lg"
              style={{ width: "100%", justifyContent: "center", borderRadius: 10 }}
            >
              {submitState === "checking" ? (
                <><span className="spinner" />Checking…</>
              ) : "Sign in"}
            </button>
          </form>

          {/* Warning note */}
          <div style={{
            background: "var(--panel)", borderLeft: "4px solid var(--warn)",
            borderRadius: 8, padding: "11px 13px",
            border: "1px solid var(--line)",
          }}>
            <p style={{ margin: 0, fontSize: 12, color: "var(--muted)", lineHeight: 1.5 }}>
              Break-glass sign-in is recorded in the audit log with IP and device, and notifies the platform on-call. Use SSO unless it's down.
            </p>
          </div>

          {/* Footer */}
          <p style={{ margin: 0, fontSize: 12.5, color: "var(--faint)", textAlign: "center" }}>
            No account? Ask your team lead to add you.{" "}
            <a href="#" style={{ color: "var(--muted)", textDecoration: "none" }}>Status</a>
            {" · "}
            <a href="#" style={{ color: "var(--muted)", textDecoration: "none" }}>Docs</a>
          </p>
        </div>
      </div>
    </div>
  );
}
