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
// 13. Create ArgoCD Application — dev (+ platform infra ArgoCD app if infra reqs exist)
// 14. Create ArgoCD Application — uat (+ platform infra ArgoCD app if infra reqs exist)
// 15. Create ArgoCD Application — prod (+ platform infra ArgoCD app if infra reqs exist;
//     direct EnsureDatabase only when CNPG infra req is NOT declared)

package provisioner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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
	"Create ArgoCD Application — prod",                      // 15
	"Configure Vault secrets path",                          // 16
	"Register Dependency-Track project + CVE notifications", // 17
}

// ProvisionInput carries everything the orchestrator needs beyond what is
// already stored in db.Project.
type ProvisionInput struct {
	Project         *db.Project
	Environments    []*db.Environment // pre-created dev / uat / prod rows
	GitNamespace    string            // namespace where the source repo lives, e.g. "restaurant-pos"
	ApplicationSlug string            // parent application slug, e.g. "restaurant-pos"
	OrgID           uuid.UUID         // needed to query cluster registry
}

// Orchestrator executes the 17-step provisioning flow for one project at a time.
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
	vault     plugin.VaultProvider
	dt        plugin.DependencyTrackProvider
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
	vault plugin.VaultProvider,
	dt plugin.DependencyTrackProvider,
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
		vault:     vault,
		dt:        dt,
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

	// ── Pre-load infra requirements and service dependencies ─────────────────
	// Loaded before step 12 so results are available to both the manifest commit
	// (step 12) and the ArgoCD Application creation (steps 13–15).
	var tmplInfraReqs []tmpl.InfraRequirement
	var tmplDeps []tmpl.ServiceDep
	if infraDBReqs, err := o.database.ListServiceInfraRequirements(ctx, projectID); err != nil {
		slog.Warn("provisioner: could not load infra requirements (non-fatal)", "err", err)
	} else {
		for _, req := range infraDBReqs {
			var cfg map[string]string
			if len(req.Config) > 0 {
				_ = json.Unmarshal(req.Config, &cfg)
			}
			if cfg == nil {
				cfg = map[string]string{}
			}
			tmplInfraReqs = append(tmplInfraReqs, tmpl.InfraRequirement{
				ServiceType: req.ServiceType,
				Config:      cfg,
			})
		}
	}
	if dbDeps, err := o.database.ListServiceDependencies(ctx, projectID); err != nil {
		slog.Warn("provisioner: could not load service dependencies (non-fatal)", "err", err)
	} else {
		for _, dep := range dbDeps {
			targetProj, err := o.database.GetProject(ctx, dep.ToProject)
			if err != nil {
				slog.Warn("provisioner: could not resolve dep slug (skipping)", "to_project", dep.ToProject, "err", err)
				continue
			}
			tmplDeps = append(tmplDeps, tmpl.ServiceDep{
				TargetSlug: targetProj.Slug,
				Port:       dep.Port,
			})
		}
	}
	// Resolve cluster registry data for each environment tier before step 12.
	// Each env may point at a different cluster with different platform service refs.
	// Falls back to flat config values when no cluster is registered.
	envClusters := map[string]envCluster{}
	for _, env := range []string{"dev", "uat", "prod"} {
		envClusters[env] = o.clusterPlatform(ctx, input.OrgID, env)
	}

	// Load admin-edited manifest template overrides from DB (non-fatal).
	// If a template has been customised in the admin UI, that content is used;
	// otherwise the compiled-in default in DefaultTemplateContent() is used.
	manifestOverrides := map[string]string{}
	if dbTmpls, err := o.database.ListManifestTemplates(ctx); err != nil {
		slog.Warn("provisioner: could not load manifest template overrides (using defaults)", "err", err)
	} else {
		for _, t := range dbTmpls {
			if t.Content != "" {
				manifestOverrides[t.Name] = t.Content
			}
		}
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
		// Load language profile once — same for all three envs
		langDB, _ := o.database.GetLanguageProfile(ctx, p.BuildTool)
		lang := tmpl.LangProfile{
			LivenessDelay:  langDB.LivenessDelay,
			ReadinessDelay: langDB.ReadinessDelay,
			ExtraEnv:       langDB.ExtraEnv,
		}

		for _, env := range []string{"dev", "uat", "prod"} {
			namespace := p.Slug + "-" + env
			ingressHost := p.Slug + "-" + env + "." + o.cfg.IngressBaseDomain

			// Load environment profile for resource quotas
			envProfile, _ := o.database.GetEnvironmentProfile(ctx, env)
			res := tmpl.ResourceSpec{
				Replicas: envProfile.Replicas,
				CPUReq:   envProfile.CPURequest,
				MemReq:   envProfile.MemRequest,
				CPULim:   envProfile.CPULimit,
				MemLim:   envProfile.MemLimit,
			}

			kFiles := o.templates.KustomizeManifests(tmpl.ManifestInput{
				AppName:     p.Slug,
				Namespace:   namespace,
				Environment: env,
				Image:       image,
				IngressHost: ingressHost,
				Port:        p.Port,
				HealthPath:  p.HealthPath,
				ServiceKind: p.ServiceKind,
				Resources:   res,
				Lang:        lang,
				InfraReqs:   tmplInfraReqs,
				Deps:        tmplDeps,
				Platform:    envClusters[env].platform,
				VaultMount:    o.cfg.VaultKVMount,
				VaultK8sMount: o.cfg.VaultK8sAuthMount,
				UseVSO:        o.cfg.VaultUseVSO,
			}, manifestOverrides)

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

		// Commit the Helm App-of-Apps chart to the dev branch.
		// Non-fatal — the service is fully operational without it; the chart
		// is a convenience artifact for one-command cluster installs.
		helmFiles := tmpl.HelmChart(tmpl.HelmChartInput{
			AppName:  p.Slug,
			RepoURL:  manifestRepoURL,
			HasInfra: len(tmplInfraReqs) > 0,
		})
		chartFiles := make([]plugin.CommitFile, 0, len(helmFiles))
		for path, content := range helmFiles {
			chartFiles = append(chartFiles, plugin.CommitFile{Path: path, Content: content, Action: "upsert"})
		}
		if chartErr := o.git.CommitFiles(ctx, plugin.CommitFilesInput{
			RepoPath:    manifestRepoPath,
			Branch:      "dev",
			Message:     "chore: DevPortal bootstrap — " + p.Slug + " Helm App-of-Apps chart [skip ci]",
			AuthorName:  o.cfg.BotName,
			AuthorEmail: o.cfg.BotEmail,
			Files:       chartFiles,
		}); chartErr != nil {
			slog.Warn("provisioner: helm chart commit failed (non-fatal)", "err", chartErr)
		}

		return nil
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Steps 13–15: ArgoCD + databases per environment ──────────────────────
	envs := envByName(input.Environments)

	// When the service declared a CNPG infra requirement the Database CR is
	// committed to git (step 12) and the "-platform" ArgoCD Application syncs
	// it. The CNPG operator creates the database and user automatically, so the
	// direct EnsureDatabase/EnsureUser SQL calls must be skipped to avoid
	// racing the operator or creating resources in the wrong cluster.
	hasCNPG := false
	for _, req := range tmplInfraReqs {
		if req.ServiceType == "cnpg" {
			hasCNPG = true
			break
		}
	}

	for stepIdx, envName := range []string{"dev", "uat", "prod"} {
		idx := 13 + stepIdx // steps 13, 14, 15
		env := envs[envName]
		appName := p.Slug + "-" + envName
		namespace := p.Slug + "-" + envName
		ingressHost := p.Slug + "-" + envName + "." + o.cfg.IngressBaseDomain

		apiServer := envClusters[envName].apiServer
		if err := o.step(ctx, projectID, idx, func() error {
			// Skip gracefully when no k8s cluster / ArgoCD is configured.
			// Manifests are still committed to git; wire ArgoCD when a cluster is available.
			if o.cfg.ArgoCDURL == "" {
				slog.Info("provisioner: ArgoCD not configured — skipping GitOps step",
					"project", p.Name, "env", envName)
				return nil
			}

			// ArgoCD Application — service overlay (Deployment, Service, Ingress, NetworkPolicy)
			if e := o.gitops.CreateApplication(ctx, plugin.CreateAppInput{
				Name:           appName,
				Namespace:      namespace,
				RepoURL:        manifestRepoURL,
				Path:           p.Slug + "/overlays/" + envName, // Kustomize overlay per env
				TargetRevision: envName,                          // branch per environment: dev / uat / prod
				Server:         apiServer,
				AutoSync:       true,
			}); e != nil {
				return fmt.Errorf("argocd %s: %w", envName, e)
			}

			// ArgoCD Application — platform operator CRs (CNPG, Kafka, RabbitMQ)
			// Only created when the service declared infra requirements.
			if len(tmplInfraReqs) > 0 {
				platformAppName := appName + "-platform"
				if e := o.gitops.CreateApplication(ctx, plugin.CreateAppInput{
					Name:           platformAppName,
					Namespace:      "argocd",
					RepoURL:        manifestRepoURL,
					Path:           p.Slug + "/infra/" + envName,
					TargetRevision: envName,
					Server:         apiServer,
					AutoSync:       true,
				}); e != nil {
					slog.Warn("provisioner: platform ArgoCD app creation failed (non-fatal)", "app", platformAppName, "err", e)
				}
			}

			// Provision app database — direct SQL path only (non-CNPG).
			// Services that selected CNPG get a committed Database CR instead;
			// the CNPG operator creates the DB, user, and grants automatically.
			dbName := strings.ReplaceAll(p.Slug, "-", "_") + "_" + envName
			dbUser := dbName + "_user"
			if !hasCNPG {
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

	// ── Step 16: Configure Vault secrets path ────────────────────────────────
	// For each environment: ensure the KV path exists with placeholder keys,
	// write an ACL policy, and create the Kubernetes auth role binding the
	// service's ServiceAccount to that policy.
	// Non-fatal when VAULT_URL is not configured — the step marks done with a
	// warning so provisioning completes and the platform team wires Vault later.
	if err := o.step(ctx, projectID, 16, func() error {
		if o.cfg.VaultURL == "" {
			slog.Info("provisioner: Vault not configured — skipping step 16", "project", p.Name)
			return nil
		}
		if len(tmplInfraReqs) == 0 {
			slog.Info("provisioner: no infra requirements — skipping Vault step", "project", p.Name)
			return nil
		}

		// Build the placeholder KV data map from declared infra requirements.
		kvData := map[string]string{}
		for _, req := range tmplInfraReqs {
			switch req.ServiceType {
			case "cnpg":
				kvData["DB_PASSWORD"] = "" // filled by platform team after CNPG operator sets password
			case "kafka":
				kvData["KAFKA_SASL_PASSWORD"] = ""
			case "rabbitmq":
				kvData["RABBITMQ_PASSWORD"] = ""
			case "redis":
				kvData["REDIS_PASSWORD"] = "" // nopass ACL user — may remain empty
			case "minio":
				kvData["MINIO_ACCESS_KEY"] = ""
				kvData["MINIO_SECRET_KEY"] = ""
			}
		}

		for _, envName := range []string{"dev", "uat", "prod"} {
			kvPath := "devportal/" + p.Slug + "/" + envName
			policyName := "devportal-" + p.Slug + "-" + envName
			roleName := policyName
			saNamespace := p.Slug + "-" + envName

			policyHCL := fmt.Sprintf(`path "%s/data/%s" { capabilities = ["read"] }`,
				o.cfg.VaultKVMount, kvPath)

			if err := o.vault.EnsureKVSecret(ctx, o.cfg.VaultKVMount, kvPath, kvData); err != nil {
				return fmt.Errorf("vault KV %s: %w", envName, err)
			}
			if err := o.vault.EnsurePolicy(ctx, policyName, policyHCL); err != nil {
				return fmt.Errorf("vault policy %s: %w", envName, err)
			}
			if err := o.vault.EnsureKubernetesRole(ctx, o.cfg.VaultK8sAuthMount, roleName, saNamespace, p.Slug, []string{policyName}); err != nil {
				return fmt.Errorf("vault k8s role %s: %w", envName, err)
			}
		}
		return nil
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── Step 17: Dependency-Track project + CVE email notifications ──────────
	// Creates (or verifies) the DT project and writes a notification rule that
	// emails all current application members when a new vulnerability is found.
	// Non-fatal — skipped when DEPENDENCY_TRACK_API_KEY is not configured.
	if err := o.step(ctx, projectID, 17, func() error {
		if o.dt == nil || o.cfg.DependencyTrackAPIKey == "" {
			slog.Info("provisioner: Dependency-Track not configured — skipping step 17", "project", p.Name)
			return nil
		}

		dtUUID, err := o.dt.EnsureProject(ctx, p.Slug)
		if err != nil {
			return fmt.Errorf("dt ensure project: %w", err)
		}
		if dtUUID == "" {
			return nil // DT skipped gracefully (empty URL)
		}

		// Gather application member emails for the notification rule.
		var emails []string
		if p.ApplicationID != nil {
			members, _ := o.database.ListApplicationMembers(ctx, *p.ApplicationID)
			emails = make([]string, 0, len(members))
			for _, m := range members {
				emails = append(emails, m.Email)
			}
		}

		return o.dt.EnsureEmailNotification(ctx, dtUUID, p.Slug+"-vuln", emails)
	}); err != nil {
		o.fail(ctx, projectID)
		return err
	}

	// ── All steps done ────────────────────────────────────────────────────────
	_ = o.database.UpdateProjectStatus(ctx, projectID, "active")
	o.hub.Broadcast(projectID, StepEvent{Done: true, Status: "done", Label: "Provisioning complete"})
	slog.Info("provisioning complete", "project", p.Name, "id", projectID)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// envCluster bundles the resolved platform refs and k8s API server for one
// environment tier. Populated from the cluster registry; falls back to flat config.
type envCluster struct {
	platform  tmpl.PlatformRefs
	apiServer string // k8s cluster URL passed as ArgoCD Application destination.server
}

// clusterPlatform queries the cluster registry for the given environment and
// builds a PlatformRefs from cluster_platform_services JSONB rows.
// When no cluster is registered for that environment it returns the flat-config
// defaults so provisioning keeps working without any cluster registered.
func (o *Orchestrator) clusterPlatform(ctx context.Context, orgID uuid.UUID, env string) envCluster {
	defaults := tmpl.PlatformRefs{
		CNPGClusterName:          o.cfg.CNPGClusterName,
		CNPGClusterNamespace:     o.cfg.CNPGClusterNamespace,
		KafkaClusterName:         o.cfg.KafkaClusterName,
		KafkaClusterNamespace:    o.cfg.KafkaClusterNamespace,
		RabbitMQClusterName:      o.cfg.RabbitMQClusterName,
		RabbitMQClusterNamespace: o.cfg.RabbitMQClusterNamespace,
		RedisNamespace:           o.cfg.RedisNamespace,
		MinIONamespace:           o.cfg.MinIONamespace,
	}
	result := envCluster{platform: defaults, apiServer: "https://kubernetes.default.svc"}

	if orgID == (uuid.UUID{}) {
		return result
	}

	cluster, err := o.database.GetClusterByEnvironment(ctx, orgID, env)
	if err != nil {
		slog.Info("provisioner: no cluster registered for env — using config defaults",
			"env", env, "org", orgID)
		return result
	}

	if cluster.APIEndpoint != "" {
		result.apiServer = cluster.APIEndpoint
	}

	svcs, err := o.database.ListEnabledClusterPlatformServices(ctx, cluster.ID)
	if err != nil {
		slog.Warn("provisioner: could not load cluster platform services — using config defaults",
			"cluster", cluster.Name, "err", err)
		return result
	}

	p := defaults
	for _, svc := range svcs {
		var cfg map[string]string
		if len(svc.Config) > 0 {
			_ = json.Unmarshal(svc.Config, &cfg)
		}
		if cfg == nil {
			continue
		}
		switch svc.ServiceType {
		case "cnpg":
			if v := cfg["cluster_name"]; v != "" { p.CNPGClusterName = v }
			if v := cfg["namespace"]; v != "" { p.CNPGClusterNamespace = v }
		case "kafka":
			if v := cfg["cluster_name"]; v != "" { p.KafkaClusterName = v }
			if v := cfg["namespace"]; v != "" { p.KafkaClusterNamespace = v }
		case "rabbitmq":
			if v := cfg["cluster_name"]; v != "" { p.RabbitMQClusterName = v }
			if v := cfg["namespace"]; v != "" { p.RabbitMQClusterNamespace = v }
		case "redis":
			if v := cfg["namespace"]; v != "" { p.RedisNamespace = v }
			if v := cfg["admin_secret"]; v != "" { p.RedisAdminSecret = v }
		case "minio":
			if v := cfg["namespace"]; v != "" { p.MinIONamespace = v }
			if v := cfg["admin_secret"]; v != "" { p.MinIOAdminSecret = v }
		}
	}
	result.platform = p
	slog.Info("provisioner: resolved cluster from registry",
		"env", env, "cluster", cluster.Name, "api_server", result.apiServer)
	return result
}

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
