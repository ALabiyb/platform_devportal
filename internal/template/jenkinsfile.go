// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// Package template renders Jenkinsfiles and Dockerfiles for provisioned services.
//
// Architecture:
//   - Default templates live in defaults/ as real files (syntax-highlighted,
//     diffable, copy-pasteable). They are compiled into the binary via go:embed
//     so there is no runtime file dependency.
//   - At startup, defaults are seeded into the pipeline_templates DB table using
//     ON CONFLICT DO NOTHING — admin edits in the DB are always preserved.
//   - The orchestrator reads the current template from DB at provision time,
//     then calls RenderJenkinsfile to substitute %%%MARKER%%% tokens.
package template

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// ── Embedded default files ────────────────────────────────────────────────────
// Each file is a real file in defaults/ — edit it directly, no Go rebuild needed
// to change what gets seeded (the binary embeds the file at compile time).

//go:embed defaults/Jenkinsfile.template
var defaultJenkinsfile string

//go:embed defaults/maven.Dockerfile
var dockerfileMaven string

//go:embed defaults/gradle.Dockerfile
var dockerfileGradle string

//go:embed defaults/go.Dockerfile
var dockerfileGo string

//go:embed defaults/nodejs-express.Dockerfile
var dockerfileNodejs string

//go:embed defaults/nextjs.Dockerfile
var dockerfileNextjs string

//go:embed defaults/python-fastapi.Dockerfile
var dockerfilePython string

//go:embed defaults/dotnet.Dockerfile
var dockerfileDotnet string

//go:embed defaults/flutter-web.Dockerfile
var dockerfileFlutter string

//go:embed defaults/auto.Dockerfile
var dockerfileAuto string

// ── Generator ─────────────────────────────────────────────────────────────────

// Generator holds the config values baked into every rendered artifact.
type Generator struct {
	cfg *config.Config
}

// New constructs a Generator.
func New(cfg *config.Config) *Generator {
	return &Generator{cfg: cfg}
}

// ── Input / rendering ─────────────────────────────────────────────────────────

// JenkinsfileInput carries the per-service values substituted into the template.
// Infrastructure values (registry URL, credentials IDs) come from cfg, not here.
type JenkinsfileInput struct {
	AppName            string // service slug, e.g. "api-gateway"
	HarborProject      string // e.g. "restaurant-pos-system/api-gateway"
	BuildTool          string // "maven" | "gradle" | "go" | etc.
	NotificationEmail  string
	GitRepoURL         string // filled after step 1 (repo creation)
	ManifestRepoURL    string // e.g. "http://gitea:3000/nexbridge/restaurant-pos-k8s.git"
	AppTimezone        string // e.g. "Africa/Dar_es_Salaam"
	StagingURL         string // optional — leave empty to skip DAST
	// K8sManifestPaths is kept for DB compatibility but no longer used in rendering.
	// Overlay paths are derived from AppName: <AppName>/overlays/{dev,uat,prod}
	K8sManifestPaths string
	EngagementID       int    // DefectDojo engagement ID (0 = not yet created)
}

// RenderJenkinsfile substitutes all %%%MARKER%%% tokens in templateStr.
// templateStr comes from the pipeline_templates DB row (editable at runtime).
// Infrastructure markers come from cfg; per-project markers come from input.
func (g *Generator) RenderJenkinsfile(templateStr string, input JenkinsfileInput) string {
	markers := map[string]string{
		// Per-project — filled at service creation time
		"%%%PROJECT_NAME%%%":                  input.AppName,
		"%%%IMAGE_NAME%%%":                    input.AppName,
		"%%%HARBOR_PROJECT%%%":                input.HarborProject,
		"%%%NOTIFICATION_EMAIL%%%":            input.NotificationEmail,
		"%%%GIT_REPO_URL%%%":                  input.GitRepoURL,
		"%%%APP_TIMEZONE%%%":                  timezone(input.AppTimezone),
		"%%%K8S_MANIFEST_SECTION%%%":          k8sSection(input, g.cfg.GitCredentialsID),
		"%%%BUILD_TOOL_LINE%%%":               buildToolLine(input.BuildTool),
		"%%%DEFECTDOJO_ENGAGEMENT_SECTION%%%": engagementSection(input.EngagementID),
		"%%%STAGING_URL_SECTION%%%":           stagingSection(input.StagingURL),

		// Infrastructure — from .env, never from form input
		"%%%REGISTRY_URL%%%":            g.cfg.RegistryURL,
		"%%%REGISTRY_CREDENTIALS_ID%%%": g.cfg.RegistryCredentialsID,
		"%%%GIT_CREDENTIALS_ID%%%":      g.cfg.GitCredentialsID,
		"%%%SHARED_LIBRARY_URL%%%":      g.cfg.SharedLibraryURL,
		"%%%DEFECTDOJO_URL%%%":          g.cfg.DefectDojoURL,
		"%%%DEPENDENCY_TRACK_URL%%%":          g.cfg.DependencyTrackURL,
		"%%%DEPENDENCY_TRACK_API_KEY_ID%%%":   g.cfg.DependencyTrackAPIKeyID,
	}

	result := templateStr
	for marker, value := range markers {
		result = strings.ReplaceAll(result, marker, value)
	}
	return result
}

// ── Marker helpers ────────────────────────────────────────────────────────────

func timezone(tz string) string {
	if tz == "" {
		return "Africa/Dar_es_Salaam"
	}
	return tz
}

func buildToolLine(buildTool string) string {
	if buildTool == "" || buildTool == "auto" {
		return "// BUILD_TOOL auto-detected from project files (pom.xml / package.json / go.mod)"
	}
	return fmt.Sprintf("BUILD_TOOL = '%s'", buildTool)
}

func engagementSection(id int) string {
	if id > 0 {
		return fmt.Sprintf("DEFECTDOJO_ENGAGEMENT_ID = '%d'", id)
	}
	return "// DEFECTDOJO_ENGAGEMENT_ID = ''  // set after DevPortal Step 11 completes"
}

func stagingSection(stagingURL string) string {
	if stagingURL != "" {
		return fmt.Sprintf("STAGING_URL = '%s'", stagingURL)
	}
	return "// STAGING_URL = ''  // set a staging URL to enable DAST scanning"
}

func k8sSection(input JenkinsfileInput, gitCredID string) string {
	if input.ManifestRepoURL != "" {
		return fmt.Sprintf(
			"K8S_MANIFEST_REPO_URL       = '%s'\n"+
				"        K8S_MANIFEST_CREDENTIALS_ID = '%s'\n"+
				"        K8S_MANIFEST_BRANCH         = 'dev'\n"+
				"        K8S_MANIFEST_UAT_BRANCH     = 'uat'\n"+
				"        K8S_MANIFEST_PROD_BRANCH    = 'prod'\n"+
				"        // Kustomize overlay directories — Jenkins runs 'kustomize edit set image' here\n"+
				"        K8S_OVERLAY_DEV             = '%[3]s/overlays/dev'\n"+
				"        K8S_OVERLAY_UAT             = '%[3]s/overlays/uat'\n"+
				"        K8S_OVERLAY_PROD            = '%[3]s/overlays/prod'",
			input.ManifestRepoURL, gitCredID, input.AppName,
		)
	}
	return fmt.Sprintf(
		"// K8S_MANIFEST_REPO_URL       = ''  // devportal creates this at step 12\n"+
			"        // K8S_MANIFEST_CREDENTIALS_ID = '%s'",
		gitCredID,
	)
}

// ── Default seed data ─────────────────────────────────────────────────────────

// DefaultJenkinsfileTemplate returns the shared pipeline-init Jenkinsfile template.
// Called only to seed pipeline_templates on first run.
func DefaultJenkinsfileTemplate() string {
	return defaultJenkinsfile
}

// DefaultDockerfile returns the production Dockerfile for a build tool.
// Called only to seed pipeline_templates on first run.
func DefaultDockerfile(buildTool string) string {
	switch strings.ToLower(buildTool) {
	case "gradle":
		return dockerfileGradle
	case "go":
		return dockerfileGo
	case "nodejs-express", "nodejs":
		return dockerfileNodejs
	case "nextjs":
		return dockerfileNextjs
	case "python-fastapi", "python":
		return dockerfilePython
	case "dotnet":
		return dockerfileDotnet
	case "flutter-web":
		return dockerfileFlutter
	default: // maven, auto
		return dockerfileAuto
	}
}

// SeedBuildTools returns all build tool keys that need a DB row.
func SeedBuildTools() []string {
	return []string{
		"maven", "gradle", "go", "nodejs-express",
		"nextjs", "python-fastapi", "dotnet", "flutter-web", "auto",
	}
}

// LangProfileSeed holds the default values for one language profile.
type LangProfileSeed struct {
	BuildTool      string
	DisplayName    string
	LivenessDelay  int
	ReadinessDelay int
	ExtraEnv       map[string]string
}

// DefaultLanguageProfiles returns the platform-standard probe timing and env vars
// per build tool. Platform team can override any of these via the admin UI.
func DefaultLanguageProfiles() []LangProfileSeed {
	return []LangProfileSeed{
		{
			BuildTool: "maven", DisplayName: "Java — Maven",
			LivenessDelay: 60, ReadinessDelay: 30,
			ExtraEnv: map[string]string{"JAVA_TOOL_OPTIONS": "-Xms256m -Xmx768m"},
		},
		{
			BuildTool: "gradle", DisplayName: "Java — Gradle",
			LivenessDelay: 60, ReadinessDelay: 30,
			ExtraEnv: map[string]string{"JAVA_TOOL_OPTIONS": "-Xms256m -Xmx768m"},
		},
		{
			BuildTool: "go", DisplayName: "Go",
			LivenessDelay: 10, ReadinessDelay: 5,
			ExtraEnv: map[string]string{},
		},
		{
			BuildTool: "nodejs-express", DisplayName: "Node.js",
			LivenessDelay: 20, ReadinessDelay: 10,
			ExtraEnv: map[string]string{"NODE_ENV": "production"},
		},
		{
			BuildTool: "nextjs", DisplayName: "Next.js",
			LivenessDelay: 25, ReadinessDelay: 15,
			ExtraEnv: map[string]string{"NODE_ENV": "production"},
		},
		{
			BuildTool: "python-fastapi", DisplayName: "Python — FastAPI",
			LivenessDelay: 20, ReadinessDelay: 10,
			ExtraEnv: map[string]string{},
		},
		{
			BuildTool: "dotnet", DisplayName: ".NET",
			LivenessDelay: 30, ReadinessDelay: 15,
			ExtraEnv: map[string]string{"ASPNETCORE_ENVIRONMENT": "Production"},
		},
		{
			BuildTool: "flutter-web", DisplayName: "Flutter Web",
			LivenessDelay: 10, ReadinessDelay: 5,
			ExtraEnv: map[string]string{},
		},
		{
			BuildTool: "auto", DisplayName: "Auto-detect",
			LivenessDelay: 30, ReadinessDelay: 10,
			ExtraEnv: map[string]string{},
		},
	}
}
