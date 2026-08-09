// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// orchestrator.go runs the 15-step provisioning flow for a new developer project.
// Each step updates the DB and broadcasts a StepEvent to all SSE subscribers.
// On any step failure the remaining steps are left "pending" and the project
// status is set to "failed". The whole flow runs in a goroutine so the HTTP
// handler returns 201 immediately and the browser watches live progress via SSE.
//
// Step sequence:
//  1. Create source repository
//  2. Commit initial project files (Jenkinsfile, VERSION, Dockerfile)
//  3. Configure GitLab webhook → Jenkins
//  4. Protect main branch
//  5. Ensure Jenkins team folder
//  6. Create Jenkins multibranch pipeline job
//  7. Trigger Jenkins branch scan
//  8. Ensure Harbor registry project
//  9. Create Harbor robot account (Jenkins push access)
// 10. Ensure DefectDojo product
// 11. Create DefectDojo CI/CD engagement
// 12. Create K8s manifest repository + commit manifests (dev / uat / prod)
// 13. Create ArgoCD Application — dev
// 14. Create ArgoCD Application — uat
// 15. Create ArgoCD Application — prod + provision app databases

package provisioner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/ALabiyb/platform_devportal/internal/config"
	"github.com/ALabiyb/platform_devportal/internal/db"
	"github.com/ALabiyb/platform_devportal/internal/plugin"
	tmpl "github.com/ALabiyb/platform_devportal/internal/template"
)

// StepLabels are the human-readable names shown in the UI for each of the 15 steps.
var StepLabels = []string{
	"Create source repository",                         // 1
	"Commit initial project files",                     // 2
	"Configure repository webhook",                     // 3
	"Protect main branch",                              // 4
	"Ensure Jenkins team folder",                       // 5
	"Create Jenkins multibranch pipeline job",          // 6
	"Trigger Jenkins branch scan",                      // 7
	"Ensure Harbor registry project",                   // 8
	"Create Harbor robot account",                      // 9
	"Ensure DefectDojo product",                        // 10
	"Create DefectDojo CI/CD engagement",               // 11
	"Create manifest repository + commit K8s YAMLs",   // 12
	"Create ArgoCD Application — dev",                  // 13
	"Create ArgoCD Application — uat",                  // 14
	"Create ArgoCD Application — prod + databases",     // 15
}

// ProvisionInput carries everything the orchestrator needs beyond what is
// already stored in db.Project.
type ProvisionInput struct {
	Project         *db.Project
	Environments    []*db.Environment // pre-created dev / uat / prod rows
	GitNamespace    string            // namespace where the source repo lives, e.g. "restaurant-pos"
	ApplicationSlug string            // parent application slug, e.g. "restaurant-pos"
}

// Orchestrator executes the 15-step provisioning flow for one project at a time.
// Create one at startup and reuse it — it is safe for concurrent Provision calls.
type Orchestrator struct {
	database  *db.DB
	hub       *Hub
	git       plugin.GitProvider
	ci        plugin.CIProvider
	registry  plugin.RegistryProvider
	security  plugin.SecurityProvider
	gitops    plugin.GitOpsProvider
	dbprov    plugin.DBProvider
	cfg       *config.Config
	templates *tmpl.Generator
}

// New constructs an Orchestrator with all its plugin dependencies.
func New(
	database *db.DB,
	hub *Hub,
	git plugin.GitProvider,
	ci plugin.CIProvider,
	registry plugin.RegistryProvider,
	security plugin.SecurityProvider,
	gitops plugin.GitOpsProvider,
	dbprov plugin.DBProvider,
	cfg *config.Config,
	templates *tmpl.Generator,
) *Orchestrator {
	return &Orchestrator{
		database:  database,
		hub:       hub,
		git:       git,
		ci:        ci,
		registry:  registry,
		security:  security,
		gitops:    gitops,
		dbprov:    dbprov,
		cfg:       cfg,
		templates: templates,
	}
}

// Provision runs the 15-step flow for input.Project.
// Blocks until provisioning completes or fails; always call from a goroutine or worker.
// Returns a non-nil error if any step failed; the project status is already set to
// "failed" in the DB before the error is returned.
func (o *Orchestrator) Provision(ctx context.Context, input ProvisionInput) error {
	p := input.Project
	projectID := p.ID
	repoPath := input.GitNamespace + "/" + p.Slug
	manifestGroup := o.cfg.K8sManifestGroup

	// Option B: one manifest repo per application (e.g. restaurant-pos-k8s).
	// All services in the same application share this repo; each service gets
	// its own subdirectory (api-gateway/, billing/, etc.).
	appSlug := input.ApplicationSlug
	if appSlug == "" {
		appSlug = input.GitNamespace // fallback to namespace if slug not passed
	}
	manifestRepoName := appSlug + "-k8s"
	manifestRepoPath := manifestGroup + "/" + manifestRepoName

	// Resolve the base URL for the active git provider (Bug fix: was always GitLabURL).
	gitBaseURL := o.cfg.GiteaURL
	if o.cfg.GitProvider == "gitlab" {
		gitBaseURL = o.cfg.GitLabURL
	}
	manifestRepoURL := strings.TrimRight(gitBaseURL, "/") + "/" + manifestRepoPath + ".git"

	slog.Info("provisioning started", "project", p.Name, "id", projectID,
		"manifest_repo", manifestRepoPath)

	// ── Step 1: Create source repository ─────────────────────────────────────
	var repoResult *plugin.RepoResult
	if err := o.step(ctx, projectID, 1, func() error {
		var e error
		repoResult, e = o.git.EnsureRepo(ctx, plugin.CreateRepoInput{
			Name:          p.Slug,
			Description:   p.Name + " — managed by DevPortal",
			NamespacePath: input.GitNamespace,
			Visibility:    "private",
			DefaultBranch: "main",
		})
		return e
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// Persist resolved URLs to DB right after repo creation.
	_ = o.database.UpdateProjectURLs(ctx, projectID, repoResult.HTTPURL, manifestRepoURL)

	// ── Step 2: Commit initial project files ──────────────────────────────────
	// Fetch the admin-editable template from DB; fall back to embedded defaults.
	var jenkinsfileTemplate, dockerfileContent string
	if t, err := o.database.GetPipelineTemplate(ctx, p.BuildTool); err == nil {
		jenkinsfileTemplate = t.Jenkinsfile
		dockerfileContent = t.Dockerfile
	} else {
		jenkinsfileTemplate = tmpl.DefaultJenkinsfileTemplate()
		dockerfileContent = tmpl.DefaultDockerfile(p.BuildTool)
	}

	stagingURL := ""
	if p.StagingURL != nil {
		stagingURL = *p.StagingURL
	}

	// K8s manifest path: service subdirectory within the shared application repo.
	// Promotion (uat/prod) uses the same file path on different manifest branches.
	k8sManifestPaths := p.Slug + "/deployment.yaml"
	if p.K8sManifestPaths != "" {
		k8sManifestPaths = p.K8sManifestPaths
	}

	jfInput := tmpl.JenkinsfileInput{
		AppName:          p.Slug,
		HarborProject:    p.HarborProject,
		BuildTool:        p.BuildTool,
		NotificationEmail: p.NotificationEmail,
		GitRepoURL:       repoResult.HTTPURL,
		ManifestRepoURL:  manifestRepoURL,
		AppTimezone:      p.AppTimezone,
		StagingURL:       stagingURL,
		K8sManifestPaths: k8sManifestPaths,
		// EngagementID: 0 — filled after Step 11; Jenkinsfile re-committed then.
	}

	jf := o.templates.RenderJenkinsfile(jenkinsfileTemplate, jfInput)
	_ = o.database.SaveGeneratedJenkinsfile(ctx, projectID, jf)

	if err := o.step(ctx, projectID, 2, func() error {
		return o.git.CommitFiles(ctx, plugin.CommitFilesInput{
			RepoPath:    repoPath,
			Branch:      "main",
			Message:     "chore: DevPortal bootstrap — Jenkinsfile, VERSION, Dockerfile",
			AuthorName:  o.cfg.BotName,
			AuthorEmail: o.cfg.BotEmail,
			Files: []plugin.CommitFile{
				{Path: "Jenkinsfile", Content: jf, Action: "upsert"},
				{Path: "VERSION", Content: "0.0.1\n", Action: "upsert"},
				{Path: "Dockerfile", Content: dockerfileContent, Action: "upsert"},
			},
		})
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 3: Configure GitLab webhook → Jenkins ────────────────────────────
	webhookURL := fmt.Sprintf("%s/multibranch-webhook-trigger/invoke?token=%s",
		strings.TrimRight(o.cfg.JenkinsPublicURL, "/"),
		p.Slug,
	)
	if err := o.step(ctx, projectID, 3, func() error {
		return o.git.EnsureWebhook(ctx, plugin.WebhookInput{
			RepoPath:    repoPath,
			URL:         webhookURL,
			SecretToken: p.Slug,
			PushEvents:  true,
			MREvents:    true,
			TagEvents:   true,
			SSLVerify:   !o.cfg.TLSSkipVerify,
		})
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 4: Protect main branch ───────────────────────────────────────────
	if err := o.step(ctx, projectID, 4, func() error {
		return o.git.EnsureProtectedBranch(ctx, repoPath, "main")
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 5: Ensure Jenkins team folder ────────────────────────────────────
	if err := o.step(ctx, projectID, 5, func() error {
		return o.ci.EnsureFolder(ctx, p.JenkinsFolder)
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 6: Create Jenkins multibranch job ────────────────────────────────
	var jobResult *plugin.JobResult
	if err := o.step(ctx, projectID, 6, func() error {
		var e error
		jobResult, e = o.ci.CreateJob(ctx, plugin.CreateJobInput{
			FolderName:    p.JenkinsFolder,
			JobName:       p.Slug,
			GitURL:        repoResult.HTTPURL,
			CredentialsID: o.cfg.GitCredentialsID,
			Description:   p.Name,
		})
		return e
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 7: Trigger Jenkins branch scan ───────────────────────────────────
	if err := o.step(ctx, projectID, 7, func() error {
		return o.ci.TriggerBranchScan(ctx, jobResult.Path)
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 8: Ensure Harbor registry project ────────────────────────────────
	// Auto-scan and 10-image retention are applied to every service project.
	if err := o.step(ctx, projectID, 8, func() error {
		return o.registry.EnsureProject(ctx, p.HarborProject, plugin.HarborProjectConfig{
			AutoScanOnPush: true,
			RetainCount:    10,
		})
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 9: Create Harbor robot account ───────────────────────────────────
	if err := o.step(ctx, projectID, 9, func() error {
		_, e := o.registry.EnsureRobotAccount(ctx, p.HarborProject, p.Slug+"-jenkins")
		// TODO Day 15: store robot credentials in the encrypted credential vault.
		return e
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 10: Ensure DefectDojo product ────────────────────────────────────
	// SLA values come from the project row (set by the lead at service creation).
	// Member emails come from the project_members table so only assigned users
	// can see this service's findings in DefectDojo.
	var productID int
	if err := o.step(ctx, projectID, 10, func() error {
		members, _ := o.database.ListProjectMemberDetails(ctx, projectID)
		emails := make([]string, 0, len(members))
		for _, m := range members {
			emails = append(emails, m.Email)
		}

		var e error
		productID, e = o.security.EnsureProduct(ctx,
			p.Name,
			p.Name+" — managed by DevPortal",
			plugin.ProductConfig{
				SLACriticalDays: p.VulnSLACritical,
				SLAHighDays:     p.VulnSLAHigh,
				SLAMediumDays:   p.VulnSLAMedium,
				SLALowDays:      p.VulnSLALow,
				MemberEmails:    emails,
			},
		)
		if e == nil {
			_ = o.database.SetDefectDojoProductID(ctx, projectID, productID)
		}
		return e
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 11: Create DefectDojo engagement ─────────────────────────────────
	// Capture ID so we can bake it into the Jenkinsfile immediately after.
	var engagementID int
	if err := o.step(ctx, projectID, 11, func() error {
		var e error
		engagementID, e = o.security.CreateEngagement(ctx, plugin.CreateEngagementInput{
			ProductID:      productID,
			Name:           p.Name + " CI/CD",
			Description:    "Automated engagement managed by DevPortal",
			EngagementType: "CI/CD",
		})
		if e != nil {
			return e
		}
		_ = o.database.SetDefectDojoEngagementID(ctx, projectID, engagementID)

		// Re-render Jenkinsfile with the real engagement ID and push the update.
		jfInput.EngagementID = engagementID
		jfUpdated := o.templates.RenderJenkinsfile(jenkinsfileTemplate, jfInput)
		_ = o.database.SaveGeneratedJenkinsfile(ctx, projectID, jfUpdated)
		return o.git.CommitFiles(ctx, plugin.CommitFilesInput{
			RepoPath:    repoPath,
			Branch:      "main",
			Message:     "chore: DevPortal — set DefectDojo engagement ID in Jenkinsfile",
			AuthorName:  o.cfg.BotName,
			AuthorEmail: o.cfg.BotEmail,
			Files: []plugin.CommitFile{
				{Path: "Jenkinsfile", Content: jfUpdated, Action: "upsert"},
			},
		})
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 12: Ensure shared manifest repo + commit service manifests ────────
	// Option B: one repo per application (e.g. restaurant-pos-k8s), branch per
	// environment (dev/uat/prod), service subdirectory per service (api-gateway/).
	// Only the dev branch is created here; uat/prod branches are created lazily
	// by promoteImage in the Jenkins pipeline when the first promotion runs.
	if err := o.step(ctx, projectID, 12, func() error {
		_, e := o.git.EnsureRepo(ctx, plugin.CreateRepoInput{
			Name:          manifestRepoName,
			Description:   appSlug + " K8s manifests — managed by DevPortal",
			NamespacePath: manifestGroup,
			Visibility:    "private",
			DefaultBranch: "dev",
		})
		if e != nil {
			return e
		}

		// Build manifests for the dev environment only; commit into the service
		// subdirectory on the dev branch. uat/prod manifests are seeded by
		// promoteImage at pipeline promotion time.
		namespace := p.Slug + "-dev"
		ingressHost := p.Slug + "-dev." + o.cfg.IngressBaseDomain
		image := strings.TrimRight(o.cfg.RegistryURL, "/") + "/" + p.HarborProject + "/" + p.Slug + ":latest"

		ms := o.templates.Manifests(tmpl.ManifestInput{
			AppName:     p.Slug,
			Namespace:   namespace,
			Environment: "dev",
			Image:       image,
			IngressHost: ingressHost,
		})

		// Service subdirectory: e.g. api-gateway/deployment.yaml
		dir := p.Slug + "/"
		files := []plugin.CommitFile{
			{Path: dir + "namespace.yaml", Content: ms.Namespace, Action: "upsert"},
			{Path: dir + "deployment.yaml", Content: ms.Deployment, Action: "upsert"},
			{Path: dir + "service.yaml", Content: ms.Service, Action: "upsert"},
			{Path: dir + "ingress.yaml", Content: ms.Ingress, Action: "upsert"},
		}
		if ms.HPA != "" {
			files = append(files, plugin.CommitFile{Path: dir + "hpa.yaml", Content: ms.HPA, Action: "upsert"})
		}

		return o.git.CommitFiles(ctx, plugin.CommitFilesInput{
			RepoPath:    manifestRepoPath,
			Branch:      "dev",
			Message:     "chore: DevPortal bootstrap — " + p.Slug + " dev manifests",
			AuthorName:  o.cfg.BotName,
			AuthorEmail: o.cfg.BotEmail,
			Files:       files,
		})
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Steps 13–15: ArgoCD + databases per environment ──────────────────────
	envs := envByName(input.Environments)

	for stepIdx, envName := range []string{"dev", "uat", "prod"} {
		idx := 13 + stepIdx // steps 13, 14, 15
		env := envs[envName]
		appName := p.Slug + "-" + envName
		namespace := p.Slug + "-" + envName
		ingressHost := p.Slug + "-" + envName + "." + o.cfg.IngressBaseDomain

		if err := o.step(ctx, projectID, idx, func() error {
			// ArgoCD Application
			if e := o.gitops.CreateApplication(ctx, plugin.CreateAppInput{
				Name:           appName,
				Namespace:      namespace,
				RepoURL:        manifestRepoURL,
				Path:           p.Slug + "/", // service subdirectory (Option B)
				TargetRevision: envName,       // branch per environment: dev / uat / prod
				AutoSync:       true,
			}); e != nil {
				return fmt.Errorf("argocd %s: %w", envName, e)
			}

			// Provision app database
			dbName := strings.ReplaceAll(p.Slug, "-", "_") + "_" + envName
			dbUser := dbName + "_user"
			dbPass, err := randomHex(16)
			if err != nil {
				return fmt.Errorf("generate db password: %w", err)
			}

			if e := o.dbprov.EnsureDatabase(ctx, dbName); e != nil {
				return fmt.Errorf("ensure database %s: %w", dbName, e)
			}
			if e := o.dbprov.EnsureUser(ctx, dbUser, dbPass); e != nil {
				return fmt.Errorf("ensure user %s: %w", dbUser, e)
			}
			if e := o.dbprov.GrantPrivileges(ctx, dbName, dbUser); e != nil {
				return fmt.Errorf("grant privileges %s→%s: %w", dbUser, dbName, e)
			}

			// Update environment row with provisioned details
			if env != nil {
				url := "https://" + ingressHost
				argoName := appName
				_ = o.database.UpdateEnvironmentDetails(ctx, env.ID, db.Environment{
					ArgoCDAppName: &argoName,
					DBName:        &dbName,
					DBUsername:    &dbUser,
					IngressURL:    &url,
					ManifestPath:  strPtr(p.Slug + "/"),
				})
				_ = o.database.UpdateEnvironmentStatus(ctx, env.ID, "active")
			}
			return nil
		}); err != nil {
			o.fail(ctx, projectID)
			return err
		}
	}

	// ── All steps done ────────────────────────────────────────────────────────
	_ = o.database.UpdateProjectStatus(ctx, projectID, "active")
	o.hub.Broadcast(projectID, StepEvent{Done: true, Status: "done", Label: "Provisioning complete"})
	slog.Info("provisioning complete", "project", p.Name, "id", projectID)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// step marks a step as running, executes fn, then marks it done or failed.
// Returns the fn error so the caller can decide whether to abort.
func (o *Orchestrator) step(ctx context.Context, projectID uuid.UUID, idx int, fn func() error) error {
	label := ""
	if idx >= 1 && idx <= len(StepLabels) {
		label = StepLabels[idx-1]
	}

	_ = o.database.UpdateProvisioningStep(ctx, projectID, idx, "running", "")
	o.hub.Broadcast(projectID, StepEvent{StepIndex: idx, Label: label, Status: "running"})

	if err := fn(); err != nil {
		detail := err.Error()
		_ = o.database.UpdateProvisioningStep(ctx, projectID, idx, "failed", detail)
		o.hub.Broadcast(projectID, StepEvent{StepIndex: idx, Label: label, Status: "failed", Detail: detail})
		slog.Error("provisioning step failed", "step", idx, "label", label, "err", err)
		return err
	}

	_ = o.database.UpdateProvisioningStep(ctx, projectID, idx, "done", "")
	o.hub.Broadcast(projectID, StepEvent{StepIndex: idx, Label: label, Status: "done"})
	return nil
}

// fail marks the project as failed and broadcasts a terminal done event.
func (o *Orchestrator) fail(ctx context.Context, projectID uuid.UUID) {
	_ = o.database.UpdateProjectStatus(ctx, projectID, "failed")
	o.hub.Broadcast(projectID, StepEvent{Done: true, Status: "failed", Label: "Provisioning failed"})
}

// envByName indexes a slice of environments by their Name field.
func envByName(envs []*db.Environment) map[string]*db.Environment {
	m := make(map[string]*db.Environment, len(envs))
	for _, e := range envs {
		if e != nil {
			m[e.Name] = e
		}
	}
	return m
}

// randomHex returns n random bytes as a lowercase hex string.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func strPtr(s string) *string { return &s }
