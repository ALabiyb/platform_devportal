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
//  1. Ensure DefectDojo product             ─┐ all external IDs gathered BEFORE
//  2. Create DefectDojo CI/CD engagement    ─┘ any git operation
//  3. Create source repository
//  4. Commit bootstrap files + create dev/uat/prod branches (zero-commit refs)
//     └─ Jenkinsfile is fully populated (real engagement ID) — this is the ONE
//        and ONLY commit DevPortal ever makes to the app repo.
//  5. Protect main branch
//  6. Ensure Jenkins team folder
//  7. Create Jenkins multibranch pipeline job
//     └─ Jenkins scans once → #1 per branch (DEVPORTAL_BOOTSTRAP, ~10 s each)
//  8. Record Jenkins pipeline job URL
//  9. Ensure Harbor registry project
// 10. Create Harbor robot account
//     └─ Steps 8–10 are API calls that take ~30 s total; by the time we finish
//        them the Jenkins initial scan is already done.
// 11. Configure repository webhook → Jenkins   ← registered HERE, after scan
//     └─ Gitea sends a ping; Jenkins scans → no new commits → zero extra builds.
//        All future developer pushes now correctly trigger CI.
// 12. Create K8s manifest repository + commit Kustomize base+overlays (dev / uat / prod)
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
	"Ensure DefectDojo product",                             // 1
	"Create DefectDojo CI/CD engagement",                    // 2
	"Create source repository",                              // 3
	"Commit bootstrap files & create dev/uat/prod branches", // 4
	"Protect main branch",                                   // 5
	"Ensure Jenkins team folder",                            // 6
	"Create Jenkins multibranch pipeline job",               // 7
	"Record Jenkins pipeline job URL",                       // 8
	"Ensure Harbor registry project",                        // 9
	"Create Harbor robot account",                           // 10
	"Configure repository webhook",                          // 11
	"Create manifest repository + commit Kustomize manifests", // 12
	"Create ArgoCD Application — dev",                       // 13
	"Create ArgoCD Application — uat",                       // 14
	"Create ArgoCD Application — prod + databases",          // 15
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

	// ── Step 1: Ensure DefectDojo product ────────────────────────────────────
	// Running BEFORE git operations so the engagement ID is available at Step 2
	// (the initial Jenkinsfile commit). No second commit to main is ever needed,
	// which means no webhook fires after the Jenkins job starts watching — each
	// branch gets exactly one DEVPORTAL_BOOTSTRAP scan build and nothing more.
	//
	// SLA values come from the project row (set by the lead at service creation).
	// Member emails come from project_members so only assigned users see findings.
	var productID int
	if err := o.step(ctx, projectID, 1, func() error {
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

	// ── Step 2: Create DefectDojo CI/CD engagement ────────────────────────────
	// On re-provision the engagement already exists — reuse its ID instead of
	// creating a duplicate. Only create when defectdojo_engagement_id is not set.
	var engagementID int
	if err := o.step(ctx, projectID, 2, func() error {
		if p.DefectDojoEngagementID != nil && *p.DefectDojoEngagementID != 0 {
			engagementID = *p.DefectDojoEngagementID
			return nil
		}
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
		return nil
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 3: Create source repository ─────────────────────────────────────
	var repoResult *plugin.RepoResult
	if err := o.step(ctx, projectID, 3, func() error {
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

	// ── Step 4: Commit bootstrap files + create branches ─────────────────────
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

	// engagementID is already set from Step 2 — bake it in now so this is the
	// only Jenkinsfile commit. No second push to main will ever happen, which
	// means no webhook fires after Jenkins is watching.
	jfInput := tmpl.JenkinsfileInput{
		AppName:           p.Slug,
		HarborProject:     p.HarborProject,
		BuildTool:         p.BuildTool,
		NotificationEmail: p.NotificationEmail,
		GitRepoURL:        repoResult.HTTPURL,
		ManifestRepoURL:   manifestRepoURL,
		AppTimezone:       p.AppTimezone,
		StagingURL:        stagingURL,
		EngagementID:      engagementID,
	}

	jf := o.templates.RenderJenkinsfile(jenkinsfileTemplate, jfInput)
	_ = o.database.SaveGeneratedJenkinsfile(ctx, projectID, jf)

	if err := o.step(ctx, projectID, 4, func() error {
		// 1. Commit all bootstrap files to main in one atomic push.
		//    Jenkinsfile is fully populated (real engagement ID from Step 2).
		//    This is the ONLY commit DevPortal ever makes to the app repo.
		//    No webhook is registered yet, so no Jenkins scan fires here.
		if err := o.git.CommitFiles(ctx, plugin.CommitFilesInput{
			RepoPath:    repoPath,
			Branch:      "main",
			Message:     "chore: DevPortal bootstrap — Jenkinsfile, VERSION, Dockerfile [skip ci]",
			AuthorName:  o.cfg.BotName,
			AuthorEmail: o.cfg.BotEmail,
			Files: []plugin.CommitFile{
				{Path: "Jenkinsfile", Content: jf, Action: "upsert"},
				{Path: "VERSION", Content: "0.0.1\n", Action: "upsert"},
				{Path: "Dockerfile", Content: dockerfileContent, Action: "upsert"},
			},
		}); err != nil {
			return err
		}

		// 2. Create dev/uat/prod as zero-commit refs pointing to main's SHA.
		//    No new commits → no webhooks → no Jenkins scan.
		for _, branch := range []string{"dev", "uat", "prod"} {
			if err := o.git.CreateBranch(ctx, repoPath, branch, "main"); err != nil {
				return fmt.Errorf("create %s branch: %w", branch, err)
			}
		}
		return nil
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 5: Protect main branch ───────────────────────────────────────────
	if err := o.step(ctx, projectID, 5, func() error {
		return o.git.EnsureProtectedBranch(ctx, repoPath, "main")
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 6: Ensure Jenkins team folder ────────────────────────────────────
	if err := o.step(ctx, projectID, 6, func() error {
		return o.ci.EnsureFolder(ctx, p.JenkinsFolder)
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 7: Create Jenkins multibranch job ────────────────────────────────
	// No webhook is registered yet. Jenkins scans on job creation and discovers
	// all four branches in one pass — each gets exactly one DEVPORTAL_BOOTSTRAP
	// build (~10 s each). No subsequent git events can trigger an extra scan.
	var jobResult *plugin.JobResult
	if err := o.step(ctx, projectID, 7, func() error {
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

	// ── Step 8: Record Jenkins pipeline job URL ───────────────────────────────
	if err := o.step(ctx, projectID, 8, func() error {
		slog.Info("provisioner: Jenkins multibranch job ready",
			"url", jobResult.URL,
			"path", jobResult.Path,
			"project", p.Slug,
		)
		return nil
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 9: Ensure Harbor registry project ────────────────────────────────
	if err := o.step(ctx, projectID, 9, func() error {
		return o.registry.EnsureProject(ctx, p.HarborProject, plugin.HarborProjectConfig{
			AutoScanOnPush: true,
			RetainCount:    10,
		})
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 10: Create Harbor robot account ─────────────────────────────────
	if err := o.step(ctx, projectID, 10, func() error {
		_, e := o.registry.EnsureRobotAccount(ctx, p.HarborProject, p.Slug+"-jenkins")
		return e
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 11: Configure repository webhook → Jenkins ───────────────────────
	// Registered HERE — after the Jenkins initial scan (Step 7) has had ~30 s to
	// complete (Steps 8–10 are API calls with real network latency). When Gitea
	// sends its creation ping, Jenkins scans but finds no new commits → zero
	// extra builds. From this point all developer pushes correctly trigger CI.
	webhookURL := fmt.Sprintf("%s/multibranch-webhook-trigger/invoke?token=%s",
		strings.TrimRight(o.cfg.JenkinsPublicURL, "/"),
		p.Slug,
	)
	if err := o.step(ctx, projectID, 11, func() error {
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

	// ── Step 12: Ensure manifest group + shared manifest repo + commit all env manifests ──
	// One repo per application (e.g. restaurant-pos-k8s), one branch per environment
	// (dev / uat / prod), one subdirectory per service (api-gateway/).
	// All three branches are seeded here so ArgoCD can sync immediately.
	if err := o.step(ctx, projectID, 12, func() error {
		// Ensure the platform manifest group exists (best-effort — already exists on re-runs).
		if _, e := o.git.EnsureGroup(ctx, manifestGroup, ""); e != nil {
			slog.Warn("provisioner: manifest group ensure failed (non-fatal)", "group", manifestGroup, "err", e)
		}

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

		image := strings.TrimRight(o.cfg.RegistryURL, "/") + "/" + p.HarborProject + "/" + p.Slug + ":latest"

		// Commit Kustomize base+overlays for all three environments.
		// uat and prod branches are created from dev via StartBranch so ArgoCD can sync immediately.
		for _, env := range []string{"dev", "uat", "prod"} {
			namespace := p.Slug + "-" + env
			ingressHost := p.Slug + "-" + env + "." + o.cfg.IngressBaseDomain
			kFiles := o.templates.KustomizeManifests(tmpl.ManifestInput{
				AppName:     p.Slug,
				Namespace:   namespace,
				Environment: env,
				Image:       image,
				IngressHost: ingressHost,
			})

			files := make([]plugin.CommitFile, 0, len(kFiles))
			for path, content := range kFiles {
				files = append(files, plugin.CommitFile{Path: path, Content: content, Action: "upsert"})
			}

			startBranch := ""
			if env != "dev" {
				startBranch = "dev" // create uat/prod branches from dev
			}

			if commitErr := o.git.CommitFiles(ctx, plugin.CommitFilesInput{
				RepoPath:    manifestRepoPath,
				Branch:      env,
				StartBranch: startBranch,
				Message:     "chore: DevPortal bootstrap — " + p.Slug + " " + env + " Kustomize manifests [skip ci]",
				AuthorName:  o.cfg.BotName,
				AuthorEmail: o.cfg.BotEmail,
				Files:       files,
			}); commitErr != nil {
				return fmt.Errorf("commit %s manifests: %w", env, commitErr)
			}
		}
		return nil
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
				Path:           p.Slug + "/overlays/" + envName, // Kustomize overlay per env
				TargetRevision: envName,                          // branch per environment: dev / uat / prod
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
					ManifestPath:  strPtr(p.Slug + "/overlays/" + envName),
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
