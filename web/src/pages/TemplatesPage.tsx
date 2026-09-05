// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState, useEffect } from "react";
import { useTemplates, useTemplate, useUpdateTemplate, PipelineTemplate } from "@/lib/api";

const LANG_COLOR: Record<string, string> = { maven: "#f87171", gradle: "#4ade80", go: "#38bdf8", node: "#fbbf24", npm: "#fbbf24", python: "#c084fc", pip: "#c084fc" };

const GW_KW = new Set(["pipeline","agent","label","environment","options","stages","stage","steps","post","always","success","failure","when","script","tools","parameters","triggers","def"]);
const GW_FN = new Set(["sh","echo","checkout","withCredentials","timeout","retry","archiveArtifacts","junit","error","dir","writeFile"]);
const DF_KW = new Set(["FROM","AS","RUN","COPY","WORKDIR","ENV","ARG","EXPOSE","ENTRYPOINT","CMD","USER","LABEL","HEALTHCHECK","VOLUME"]);

function tokenize(line: string, isGroovy: boolean): React.ReactNode[] {
  if (!line.trim()) return [" "];
  const kw = isGroovy ? GW_KW : DF_KW;
  const parts: React.ReactNode[] = [];
  let i = 0;
  const push = (txt: string, color?: string) => {
    if (!txt) return;
    parts.push(color
      ? <span key={parts.length} style={{ color, whiteSpace: "pre" }}>{txt}</span>
      : <span key={parts.length} style={{ whiteSpace: "pre" }}>{txt}</span>);
  };
  if (line.trimStart().startsWith("//") || line.trimStart().startsWith("#")) {
    push(line, "var(--faint)"); return parts;
  }
  while (i < line.length) {
    if (line[i] === "$" && line[i+1] === "{") {
      const end = line.indexOf("}", i + 2);
      if (end !== -1) { push(line.slice(i, end + 1), "var(--accent)"); i = end + 1; continue; }
    }
    if (line[i] === '"' || line[i] === "'") {
      const q = line[i]; let j = i + 1;
      while (j < line.length && line[j] !== q) j++;
      push(line.slice(i, j + 1), "var(--str)"); i = j + 1; continue;
    }
    if (/[a-zA-Z_]/.test(line[i])) {
      let j = i;
      while (j < line.length && /[a-zA-Z0-9_]/.test(line[j])) j++;
      const word = line.slice(i, j);
      if (kw.has(word)) push(word, "var(--kw)");
      else if (isGroovy && GW_FN.has(word)) push(word, "var(--fn)");
      else push(word);
      i = j; continue;
    }
    if (/\d/.test(line[i])) {
      let j = i;
      while (j < line.length && /[\d.]/.test(line[j])) j++;
      push(line.slice(i, j), "var(--num)"); i = j; continue;
    }
    push(line[i]); i++;
  }
  return parts;
}

const PLACEHOLDERS = [
  { token: "${SERVICE_NAME}", desc: "The service slug, e.g. ledger-api" },
  { token: "${APPLICATION}",  desc: "Parent application" },
  { token: "${VERSION}",      desc: "Image tag from the build" },
  { token: "${SERVICE_PORT}", desc: "Declared container port" },
  { token: "${ENVIRONMENT}",  desc: "Target env: dev, stage, prod" },
  { token: "${TEAM_SLACK}",   desc: "Owning team Slack channel" },
];

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return new Date(iso).toLocaleDateString();
}

type FileType = "Jenkinsfile" | "Dockerfile";

// ── Editor pane (no overlay tricks — just a styled textarea while editing) ────
function EditorPane({ tool, file, onFileChange }: { tool: string; file: FileType; onFileChange: (f: FileType) => void }) {
  const { data: tmpl, isLoading } = useTemplate(tool);
  const updateM = useUpdateTemplate(tool);
  const [draft, setDraft] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);

  const serverContent = file === "Jenkinsfile" ? (tmpl?.jenkinsfile ?? "") : (tmpl?.dockerfile ?? "");
  const content = draft !== null ? draft : serverContent;
  const dirty = draft !== null && draft !== serverContent;
  const lines = content.split("\n");
  const isGroovy = file === "Jenkinsfile";

  useEffect(() => { setDraft(null); setEditing(false); }, [tool, file]);

  function handleSave() {
    if (!dirty) return;
    const body = file === "Jenkinsfile" ? { jenkinsfile: draft! } : { dockerfile: draft! };
    updateM.mutate(body, { onSuccess: () => { setDraft(null); setEditing(false); } });
  }

  function handleRevert() { setDraft(null); setEditing(false); }

  if (isLoading) {
    return (
      <div style={{ flex: 1, background: "var(--panel)", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 13, color: "var(--faint)" }}>
        Loading template…
      </div>
    );
  }

  return (
    <div style={{ flex: 1, background: "var(--panel)", display: "flex", flexDirection: "column", minHeight: 0 }}>
      {/* Tab strip */}
      <div style={{ height: 52, background: "var(--bg)", borderBottom: "1px solid var(--line)", display: "flex", alignItems: "stretch", flexShrink: 0 }}>
        {(["Jenkinsfile","Dockerfile"] as const).map(f => (
          <div key={f} onClick={() => onFileChange(f)} style={{
            padding: "0 16px", cursor: "pointer", display: "flex", alignItems: "center", gap: 8,
            borderRight: "1px solid var(--line)",
            background: file === f ? "var(--panel)" : "transparent",
            borderBottom: `2px solid ${file === f ? "var(--accent)" : "transparent"}`,
            fontFamily: "JetBrains Mono,monospace", fontSize: 13,
            color: file === f ? "var(--text)" : "var(--faint)",
          }}>
            {f}
            {file === f && dirty && <span style={{ width: 6, height: 6, borderRadius: "50%", background: "var(--warn)", flexShrink: 0 }} />}
          </div>
        ))}
        <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 8, paddingRight: 16 }}>
          {!editing ? (
            <button className="btn btn-secondary btn-sm" onClick={() => setEditing(true)}>Edit</button>
          ) : (
            <>
              <button onClick={handleRevert} className="btn btn-ghost btn-sm">Revert</button>
              <button onClick={handleSave} className="btn btn-primary btn-sm" disabled={!dirty || updateM.isPending}>
                {updateM.isPending ? "Saving…" : "Save changes"}
              </button>
            </>
          )}
        </div>
      </div>

      {/* Meta bar */}
      <div style={{ height: 36, padding: "0 18px", borderBottom: "1px solid var(--line)", display: "flex", alignItems: "center", gap: 14, flexShrink: 0 }}>
        <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--faint)" }}>{tool}/templates/{file}</span>
        <span style={{ marginLeft: "auto", fontSize: 12, color: "var(--faint)" }}>
          {tmpl?.updated_at ? `Edited ${relativeTime(tmpl.updated_at)}${tmpl.updated_by ? ` by ${tmpl.updated_by}` : ""}` : ""}
        </span>
        <button className="btn btn-ghost btn-sm">History</button>
        <button className="btn btn-ghost btn-sm">Diff</button>
      </div>

      {/* Content + right rail */}
      <div style={{ flex: 1, overflow: "hidden", display: "flex", minHeight: 0 }}>
        {editing ? (
          // Edit mode: plain textarea
          <div style={{ flex: 1, display: "flex", overflow: "hidden" }}>
            <div style={{ position: "sticky", left: 0, flexShrink: 0, width: 52, background: "var(--panel)", borderRight: "1px solid var(--line)", padding: "14px 0", overflowY: "hidden", pointerEvents: "none" }}>
              {lines.map((_, i) => (
                <div key={i} style={{ height: 21, lineHeight: "21px", paddingRight: 10, textAlign: "right", fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--faint)", userSelect: "none" }}>{i + 1}</div>
              ))}
            </div>
            <textarea
              autoFocus
              value={content}
              onChange={e => setDraft(e.target.value)}
              style={{
                flex: 1, padding: "14px 18px", fontFamily: "JetBrains Mono,monospace", fontSize: 12,
                lineHeight: "21px", background: "var(--panel)", color: "var(--text)",
                border: "none", outline: "none", resize: "none", whiteSpace: "pre",
                overflowWrap: "normal", overflowX: "auto",
              }}
            />
          </div>
        ) : (
          // Read mode: syntax-highlighted
          <div style={{ flex: 1, overflow: "auto", display: "flex" }}>
            <div style={{ position: "sticky", left: 0, flexShrink: 0, width: 52, background: "var(--panel)", borderRight: "1px solid var(--line)", padding: "14px 0", zIndex: 1 }}>
              {lines.map((_, i) => (
                <div key={i} style={{ height: 21, lineHeight: "21px", paddingRight: 10, textAlign: "right", fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--faint)", userSelect: "none" }}>{i + 1}</div>
              ))}
            </div>
            <div style={{ flex: 1, padding: "14px 18px", minWidth: 0 }}>
              {lines.map((line, i) => (
                <div key={i} style={{ display: "flex", height: 21, lineHeight: "21px" }}>
                  {tokenize(line, isGroovy)}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Right rail */}
        <div style={{ width: 250, borderLeft: "1px solid var(--line)", background: "var(--bg)", display: "flex", flexDirection: "column", overflow: "hidden", flexShrink: 0 }}>
          <div style={{ padding: "14px 14px 0", flex: 1, overflowY: "auto" }}>
            <div className="overline" style={{ marginBottom: 10 }}>Available placeholders</div>
            {PLACEHOLDERS.map(p => (
              <div key={p.token}
                onClick={() => { if (editing) setDraft(prev => (prev ?? content) + p.token); }}
                style={{ padding: "7px 0", borderBottom: "1px solid var(--line)", cursor: editing ? "pointer" : "default" }}
                onMouseEnter={e => { if (editing) (e.currentTarget as HTMLElement).style.background = "var(--card)"; }}
                onMouseLeave={e => { (e.currentTarget as HTMLElement).style.background = "transparent"; }}
              >
                <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, color: "var(--accent)" }}>{p.token}</div>
                <div style={{ fontSize: 11, color: "var(--muted)", marginTop: 2 }}>{p.desc}</div>
              </div>
            ))}
          </div>
          <div style={{ padding: "12px 14px", borderTop: "1px solid var(--line)" }}>
            <div className="overline" style={{ marginBottom: 6 }}>About this template</div>
            <p style={{ fontSize: 11, color: "var(--faint)", margin: 0, lineHeight: 1.5 }}>
              Affects all services using the <strong style={{ color: "var(--muted)" }}>{tool}</strong> build tool on their next run. Click <strong style={{ color: "var(--muted)" }}>Edit</strong> to modify.
            </p>
          </div>
        </div>
      </div>

      {/* Status bar */}
      <div style={{ height: 28, background: "var(--card)", borderTop: "1px solid var(--line)", padding: "0 16px", display: "flex", alignItems: "center", gap: 16, fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, flexShrink: 0 }}>
        <span style={{ color: "var(--faint)" }}>{isGroovy ? "Groovy" : "Dockerfile"}</span>
        <span style={{ color: "var(--faint)" }}>Ln 1, Col 1 · {lines.length} lines</span>
        <span style={{ color: "var(--faint)" }}>spaces: 2</span>
        <span style={{ marginLeft: "auto", color: "var(--faint)" }}>UTF-8</span>
        <span style={{ color: "var(--faint)" }}>LF</span>
        <span style={{ color: dirty ? "var(--warn)" : editing ? "var(--accent)" : "var(--ok)" }}>
          {dirty ? "Unsaved changes" : editing ? "Editing" : "Saved"}
        </span>
      </div>
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────
export function TemplatesPage() {
  const { data: templates = [], isLoading } = useTemplates();
  const [tool, setTool] = useState<string>("");
  const [expanded, setExpanded] = useState<string>("");
  const [file, setFile] = useState<FileType>("Jenkinsfile");

  useEffect(() => {
    if (!tool && templates.length > 0) {
      setTool(templates[0].build_tool);
      setExpanded(templates[0].build_tool);
    }
  }, [templates, tool]);

  return (
    <div style={{ display: "flex", height: "calc(100vh - 52px)", overflow: "hidden" }}>
      {/* File tree */}
      <div style={{ width: 246, flexShrink: 0, background: "var(--bg)", borderRight: "1px solid var(--line)", display: "flex", flexDirection: "column", overflow: "hidden" }}>
        <div style={{ height: 52, padding: "0 14px", display: "flex", alignItems: "center", justifyContent: "space-between", borderBottom: "1px solid var(--line)", flexShrink: 0 }}>
          <span className="overline">Pipeline templates</span>
          <button className="btn btn-ghost btn-sm" style={{ width: 24, height: 24, padding: 0, fontSize: 16 }}>+</button>
        </div>
        <div style={{ flex: 1, overflowY: "auto", padding: "4px 0" }}>
          {isLoading && (
            <div style={{ padding: "14px", fontSize: 12, color: "var(--faint)" }}>Loading…</div>
          )}
          {templates.map((t: PipelineTemplate) => {
            const color = LANG_COLOR[t.build_tool] ?? "var(--faint)";
            const open = expanded === t.build_tool;
            const active = tool === t.build_tool;
            return (
              <div key={t.build_tool}>
                <div
                  onClick={() => { setExpanded(open ? "" : t.build_tool); setTool(t.build_tool); }}
                  style={{ height: 30, display: "flex", alignItems: "center", gap: 8, padding: "0 14px", cursor: "pointer",
                    background: active && !open ? "var(--accent-soft)" : "transparent" }}
                  onMouseEnter={e => { if (!active || open) (e.currentTarget as HTMLElement).style.background = "var(--card)"; }}
                  onMouseLeave={e => { (e.currentTarget as HTMLElement).style.background = active && !open ? "var(--accent-soft)" : "transparent"; }}
                >
                  <span style={{ fontSize: 9, color: "var(--faint)" }}>{open ? "▾" : "▸"}</span>
                  <span style={{ width: 7, height: 7, borderRadius: 2, background: color, flexShrink: 0 }} />
                  <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12.5, flex: 1, color: active ? "var(--accent)" : "var(--text)" }}>{t.build_tool}</span>
                </div>
                {open && (["Jenkinsfile","Dockerfile"] as const).map(f => {
                  const isActive = f === file && active;
                  return (
                    <div key={f} onClick={() => { setFile(f); setTool(t.build_tool); }}
                      style={{ height: 28, display: "flex", alignItems: "center", gap: 8, paddingLeft: 30, paddingRight: 14, cursor: "pointer",
                        background: isActive ? "var(--accent-soft)" : "transparent",
                        color: isActive ? "var(--accent)" : "var(--muted)" }}
                      onMouseEnter={e => { if (!isActive) (e.currentTarget as HTMLElement).style.background = "var(--card)"; }}
                      onMouseLeave={e => { if (!isActive) (e.currentTarget as HTMLElement).style.background = "transparent"; }}
                    >
                      <span style={{ fontSize: 11 }}>🗋</span>
                      <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, flex: 1 }}>{f}</span>
                    </div>
                  );
                })}
              </div>
            );
          })}
        </div>
        <div style={{ padding: "10px 14px", borderTop: "1px solid var(--line)", fontSize: 11, color: "var(--faint)", lineHeight: 1.5 }}>
          Templates render with service context. Placeholders use{" "}
          <span style={{ fontFamily: "JetBrains Mono,monospace", color: "var(--accent)" }}>{"${...}"}</span>.
        </div>
      </div>

      {tool ? (
        <EditorPane key={`${tool}:${file}`} tool={tool} file={file} onFileChange={setFile} />
      ) : (
        <div style={{ flex: 1, display: "flex", alignItems: "center", justifyContent: "center", fontSize: 13, color: "var(--faint)" }}>
          {isLoading ? "Loading templates…" : "Select a template from the tree."}
        </div>
      )}
    </div>
  );
}
