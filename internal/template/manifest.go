// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// manifest.go generates a Kustomize base+overlays file tree for a provisioned service.
// Every YAML file is rendered from a named Go text/template. Default template content
// is compiled into the binary via DefaultTemplateContent(); admins can override any
// template at runtime through the manifest_templates DB table and admin UI.
//
// Template variable reference: see TemplateVars.
//
// Repository layout produced (one branch per environment):
//
//	<service>/base/kustomization.yaml            — lists base resources
//	<service>/base/deployment.yaml               — env-agnostic Deployment (DEVPORTAL_IMAGE_PLACEHOLDER)
//	<service>/base/service.yaml                  — ClusterIP Service
//	<service>/overlays/<env>/kustomization.yaml  — namespace, image resolution, patches
//	<service>/overlays/<env>/namespace.yaml      — Namespace object
//	<service>/overlays/<env>/ingress.yaml        — Ingress (Traefik)
//	<service>/overlays/<env>/patch-resources.yaml — CPU/mem/replicas per env tier
//	<service>/overlays/<env>/hpa.yaml            — HPA (prod only)
//	<service>/overlays/<env>/networkpolicy.yaml  — NetworkPolicy (always — baseline default-deny + Traefik ingress + DNS egress, plus rules per declared dep/infra)
//	<service>/infra/<env>/kustomization.yaml     — operator CRs (separate ArgoCD Application)
//	<service>/infra/<env>/cnpg-database.yaml     — CNPG Database CR (if cnpg selected)
//	<service>/infra/<env>/kafka-topics.yaml      — Strimzi KafkaTopic CRs (if kafka selected)
//	<service>/infra/<env>/rabbitmq.yaml          — RabbitMQ Vhost+User CRs (if rabbitmq selected)
//
// Jenkins updates the image tag by running:
//
//	kustomize edit set image DEVPORTAL_IMAGE_PLACEHOLDER=<registry>/<project>/<service>:<build>
//
// inside the relevant overlay directory, then committing the change.
package template

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// InfraRequirement describes one piece of shared platform infrastructure
// this service needs (CNPG database, Kafka topics, etc.).
type InfraRequirement struct {
	ServiceType string            // "cnpg" | "kafka" | "rabbitmq" | "redis" | "minio"
	Config      map[string]string // type-specific config parsed from JSON
}

// ServiceDep is one outbound dependency edge: this service calls TargetSlug on Port.
// Used to generate NetworkPolicy egress rules.
type ServiceDep struct {
	TargetSlug string
	Port       int
}

// PlatformRefs holds cluster-level resource references needed to generate operator CRs
// and NetworkPolicy egress rules. Populated from config.Config at provision time.
type PlatformRefs struct {
	CNPGClusterName          string
	CNPGClusterNamespace     string
	KafkaClusterName         string
	KafkaClusterNamespace    string
	RabbitMQClusterName      string
	RabbitMQClusterNamespace string
	RedisNamespace           string
	RedisAdminSecret         string // Secret name holding redis-password key
	MinIONamespace           string
	MinIOAdminSecret         string // Secret name holding rootUser / rootPassword keys
}

// ResourceSpec carries the environment-tier resource values read from the
// environment_profiles DB table.
type ResourceSpec struct {
	Replicas int
	CPUReq   string
	MemReq   string
	CPULim   string
	MemLim   string
}

// LangProfile carries language-specific Deployment tuning read from the
// language_profiles DB table — probe timing and extra env vars.
type LangProfile struct {
	LivenessDelay  int               // initialDelaySeconds for liveness probe
	ReadinessDelay int               // initialDelaySeconds for readiness probe
	ExtraEnv       map[string]string // e.g. {"JAVA_TOOL_OPTIONS": "-Xms256m -Xmx768m"}
}

// ManifestInput holds the per-environment values needed to render K8s manifests.
type ManifestInput struct {
	AppName     string // e.g. "payment-service"
	Namespace   string // K8s namespace, e.g. "payment-service-dev"
	Environment string // "dev" | "uat" | "prod"
	Image       string // full image ref, e.g. "registry.example.com/nexbridge/payment-service:latest"
	IngressHost string // e.g. "payment-service-dev.apps.example.com"
	Port        int    // container port; 0 defaults to 8080
	HealthPath  string // health check path; "" defaults to /healthz
	ServiceKind string // "backend" | "frontend" | "worker"; defaults to "backend"
	Resources   ResourceSpec
	Lang        LangProfile
	InfraReqs   []InfraRequirement
	Deps        []ServiceDep
	Platform    PlatformRefs

	// Vault integration
	VaultMount    string // KV secrets engine mount, e.g. "secret"
	VaultK8sMount string // Kubernetes auth method mount, e.g. "kubernetes"
	UseVSO        bool   // when true, renders VaultAuth+VaultStaticSecret instead of ExternalSecret
}

// ResolvedDep is a pre-computed service dependency with env var names and URL
// ready to embed directly into templates without any string manipulation.
type ResolvedDep struct {
	TargetSlug string // e.g. "user-service"
	Port       int    // e.g. 8080
	EnvKey     string // e.g. "USER_SERVICE_URL"
	ServiceURL string // e.g. "http://user-service.user-service-dev.svc.cluster.local:8080"
}

// KustomizeFiles maps repository-relative file path → YAML content.
// Every entry is committed to the manifest repo by the orchestrator.
type KustomizeFiles map[string]string

// TemplateVars is the flat data model available inside every manifest template.
// Reference fields with {{.AppName}}, {{.Port}}, {{range .Deps}}, etc.
// All fields are safe to reference; missing values render as zero-values (empty string, 0, false).
type TemplateVars struct {
	// Service identity
	AppName     string // e.g. "payment-service"
	Namespace   string // e.g. "payment-service-dev"
	Environment string // "dev" | "uat" | "prod"
	Image       string // full image ref
	ImageName   string // registry/project/name without tag
	ImageTag    string // the tag portion only
	IngressHost string
	Port        int
	HealthPath  string

	// Resource profile (from environment_profiles)
	Replicas int
	CPUReq   string
	MemReq   string
	CPULim   string
	MemLim   string

	// Language profile (from language_profiles)
	LivenessDelay  int
	ReadinessDelay int
	ExtraEnv       map[string]string // e.g. {"JAVA_TOOL_OPTIONS": "-Xms256m"}

	// Service kind — controls manifest shape
	ServiceKind string // "backend" | "frontend" | "worker"
	IsBackend   bool
	IsFrontend  bool
	IsWorker    bool

	// Computed flags — use in conditionals: {{if .IsProd}}, {{if .HasInfra}}, etc.
	IsProd             bool
	NeedsNetworkPolicy bool
	HasInfra           bool // true when any infra requirement exists
	HasDeps            bool // true when any service-to-service dependency declared

	// Vault integration
	VaultMount    string // KV secrets engine mount, e.g. "secret"
	VaultKVPath   string // computed: "devportal/<appname>/<env>"
	VaultRole     string // computed: "devportal-<appname>-<env>"
	VaultK8sMount string // Kubernetes auth method mount, e.g. "kubernetes"
	UseVSO        bool   // renders VaultAuth+VaultStaticSecret when true

	// Platform operator references
	CNPGClusterName          string
	CNPGClusterNamespace     string
	KafkaClusterName         string
	KafkaClusterNamespace    string
	RabbitMQClusterName      string
	RabbitMQClusterNamespace string
	RedisNamespace           string
	RedisAdminSecret         string // Secret name holding redis-password key
	MinIONamespace           string
	MinIOAdminSecret         string // Secret name holding rootUser / rootPassword keys

	// Relations — iterable in templates: {{range .Deps}}, {{range .InfraReqs}}, {{range .ResolvedDeps}}
	InfraReqs    []InfraRequirement
	Deps         []ServiceDep
	ResolvedDeps []ResolvedDep // Deps with pre-computed EnvKey + ServiceURL

	// Derived CNPG names
	DBName string // e.g. "payment_service_dev"
	DBUser string // e.g. "payment_service_dev_user"

	// Derived Kafka and RabbitMQ values (pre-parsed from infra req config)
	KafkaTopics []string // topic names, split from infra req config["topics"]
	RabbitVhost string   // vhost path, e.g. "/payment-service"
}

// buildVars converts a ManifestInput into a flat TemplateVars, applying defaults
// for zero-value fields so templates never need to guard for empty values.
func buildVars(input ManifestInput) TemplateVars {
	port := input.Port
	if port == 0 {
		port = 8080
	}
	healthPath := input.HealthPath
	if healthPath == "" {
		healthPath = "/healthz"
	}

	res := input.Resources
	if res.Replicas == 0 {
		res.Replicas = 1
	}
	if res.CPUReq == "" {
		res.CPUReq = "100m"
	}
	if res.MemReq == "" {
		res.MemReq = "128Mi"
	}
	if res.CPULim == "" {
		res.CPULim = "500m"
	}
	if res.MemLim == "" {
		res.MemLim = "512Mi"
	}

	lang := input.Lang
	if lang.LivenessDelay == 0 {
		lang.LivenessDelay = 30
	}
	if lang.ReadinessDelay == 0 {
		lang.ReadinessDelay = 10
	}

	serviceKind := input.ServiceKind
	if serviceKind == "" {
		serviceKind = "backend"
	}

	vaultMount := input.VaultMount
	if vaultMount == "" {
		vaultMount = "secret"
	}
	vaultK8sMount := input.VaultK8sMount
	if vaultK8sMount == "" {
		vaultK8sMount = "kubernetes"
	}

	redisAdminSecret := input.Platform.RedisAdminSecret
	if redisAdminSecret == "" {
		redisAdminSecret = "redis-credentials"
	}
	minioAdminSecret := input.Platform.MinIOAdminSecret
	if minioAdminSecret == "" {
		minioAdminSecret = "minio-root-credentials"
	}

	imageName, imageTag := splitImage(input.Image)
	dbName := strings.ReplaceAll(input.AppName, "-", "_") + "_" + input.Environment
	dbUser := dbName + "_user"

	var kafkaTopics []string
	rabbitVhost := "/" + input.AppName
	for _, req := range input.InfraReqs {
		switch req.ServiceType {
		case "kafka":
			raw := req.Config["topics"]
			if raw == "" {
				raw = input.AppName + "-events"
			}
			for _, t := range strings.Split(raw, ",") {
				if t = strings.TrimSpace(t); t != "" {
					kafkaTopics = append(kafkaTopics, t)
				}
			}
		case "rabbitmq":
			if v := req.Config["vhost"]; v != "" {
				rabbitVhost = v
			}
		}
	}

	var resolvedDeps []ResolvedDep
	for _, dep := range input.Deps {
		envPrefix := strings.ToUpper(strings.ReplaceAll(dep.TargetSlug, "-", "_"))
		depNs := dep.TargetSlug + "-" + input.Environment
		url := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", dep.TargetSlug, depNs, dep.Port)
		resolvedDeps = append(resolvedDeps, ResolvedDep{
			TargetSlug: dep.TargetSlug,
			Port:       dep.Port,
			EnvKey:     envPrefix + "_URL",
			ServiceURL: url,
		})
	}

	return TemplateVars{
		AppName:            input.AppName,
		Namespace:          input.Namespace,
		Environment:        input.Environment,
		Image:              input.Image,
		ImageName:          imageName,
		ImageTag:           imageTag,
		IngressHost:        input.IngressHost,
		Port:               port,
		HealthPath:         healthPath,
		Replicas:           res.Replicas,
		CPUReq:             res.CPUReq,
		MemReq:             res.MemReq,
		CPULim:             res.CPULim,
		MemLim:             res.MemLim,
		LivenessDelay:      lang.LivenessDelay,
		ReadinessDelay:     lang.ReadinessDelay,
		ExtraEnv:           lang.ExtraEnv,
		ServiceKind: serviceKind,
		IsBackend:   serviceKind == "backend",
		IsFrontend:  serviceKind == "frontend",
		IsWorker:    serviceKind == "worker",
		IsProd:             input.Environment == "prod",
		// Always generate a NetworkPolicy — even a service with zero declared
		// infra/deps still gets the baseline (ingress from Traefik on its own
		// port, DNS egress). Making this conditional on Deps/InfraReqs left
		// dependency-free services with no NetworkPolicy at all, silently
		// contradicting the platform's default-deny zero-trust posture.
		NeedsNetworkPolicy: true,
		HasInfra:           len(input.InfraReqs) > 0,
		HasDeps:            len(input.Deps) > 0,
		CNPGClusterName:          input.Platform.CNPGClusterName,
		CNPGClusterNamespace:     input.Platform.CNPGClusterNamespace,
		KafkaClusterName:         input.Platform.KafkaClusterName,
		KafkaClusterNamespace:    input.Platform.KafkaClusterNamespace,
		RabbitMQClusterName:      input.Platform.RabbitMQClusterName,
		RabbitMQClusterNamespace: input.Platform.RabbitMQClusterNamespace,
		RedisNamespace:           input.Platform.RedisNamespace,
		RedisAdminSecret:         redisAdminSecret,
		MinIONamespace:           input.Platform.MinIONamespace,
		MinIOAdminSecret:         minioAdminSecret,
		VaultMount:    vaultMount,
		VaultKVPath:   "devportal/" + input.AppName + "/" + input.Environment,
		VaultRole:     "devportal-" + input.AppName + "-" + input.Environment,
		VaultK8sMount: vaultK8sMount,
		UseVSO:        input.UseVSO,
		InfraReqs:    input.InfraReqs,
		Deps:         input.Deps,
		ResolvedDeps: resolvedDeps,
		DBName:       dbName,
		DBUser:      dbUser,
		KafkaTopics: kafkaTopics,
		RabbitVhost: rabbitVhost,
	}
}

// renderFromTemplate renders a named template using text/template.
// If overrides[name] is non-empty (admin-edited content from DB), that is used;
// otherwise defaultContent (the compiled-in default) is used.
// Falls back to defaultContent if the override fails to parse.
func renderFromTemplate(name, defaultContent string, overrides map[string]string, vars TemplateVars) string {
	content := defaultContent
	if ov, ok := overrides[name]; ok && strings.TrimSpace(ov) != "" {
		content = ov
	}
	t, err := template.New(name).Parse(content)
	if err != nil {
		t, _ = template.New(name).Parse(defaultContent)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return fmt.Sprintf("# template render error for %s: %v\n", name, err)
	}
	return buf.String()
}

// DefaultTemplateContent returns the built-in Go template string for each
// named manifest template. These are seeded into the manifest_templates DB table
// and used as fallback when no admin override exists.
func DefaultTemplateContent() map[string]string {
	return map[string]string{
		"base-kustomization":      tmplBaseKustomization,
		"base-deployment":         tmplBaseDeployment,
		"base-service":            tmplBaseService,
		"overlay-kustomization":   tmplOverlayKustomization,
		"overlay-namespace":       tmplOverlayNamespace,
		"overlay-ingressroute":    tmplOverlayIngressRoute,
		"overlay-middleware":      tmplOverlayMiddleware,
		"overlay-gateway":          tmplOverlayGateway,
		"overlay-httproute":        tmplOverlayHTTPRoute,
		"overlay-infra-configmap":  tmplOverlayInfraConfigMap,
		"overlay-external-secret":  tmplOverlayExternalSecret,
		"base-serviceaccount":      tmplBaseServiceAccount,
		"overlay-resources-patch":  tmplOverlayResourcesPatch,
		"overlay-hpa":             tmplOverlayHPA,
		"overlay-networkpolicy":        tmplOverlayNetworkPolicy,
		"overlay-vault-auth":           tmplOverlayVaultAuth,
		"overlay-vault-static-secret":  tmplOverlayVaultStaticSecret,
		"infra-kustomization":     tmplInfraKustomization,
		"infra-cnpg-database":     tmplInfraCNPGDatabase,
		"infra-kafka-topics":      tmplInfraKafkaTopics,
		"infra-rabbitmq":          tmplInfraRabbitMQ,
		"infra-minio-job":         tmplInfraMinIOJob,
		"infra-redis-acl-job":     tmplInfraRedisACLJob,
	}
}

// KustomizeManifests returns the complete Kustomize file tree for one environment.
// overrides maps template name → content loaded from the manifest_templates DB table.
// Pass nil or an empty map to use all built-in defaults.
func (g *Generator) KustomizeManifests(input ManifestInput, overrides map[string]string) KustomizeFiles {
	if overrides == nil {
		overrides = map[string]string{}
	}
	defaults := DefaultTemplateContent()
	render := func(name string, vars TemplateVars) string {
		return renderFromTemplate(name, defaults[name], overrides, vars)
	}

	vars := buildVars(input)
	base    := input.AppName + "/base"
	overlay := input.AppName + "/overlays/" + input.Environment

	files := KustomizeFiles{
		base + "/kustomization.yaml":        render("base-kustomization", vars),
		base + "/deployment.yaml":           render("base-deployment", vars),
		base + "/service.yaml":              render("base-service", vars),
		base + "/serviceaccount.yaml":       render("base-serviceaccount", vars),
		overlay + "/kustomization.yaml":     render("overlay-kustomization", vars),
		overlay + "/namespace.yaml":         render("overlay-namespace", vars),
		overlay + "/patch-resources.yaml":   render("overlay-resources-patch", vars),
	}

	// Workers have no ingress — they consume queues or run on a schedule.
	if !vars.IsWorker {
		files[overlay+"/ingressroute.yaml"] = render("overlay-ingressroute", vars)
		files[overlay+"/middleware.yaml"]   = render("overlay-middleware", vars)
		files[overlay+"/gateway.yaml"]      = render("overlay-gateway", vars)
		files[overlay+"/httproute.yaml"]    = render("overlay-httproute", vars)
	}
	if vars.HasInfra || vars.HasDeps {
		files[overlay+"/infra-configmap.yaml"] = render("overlay-infra-configmap", vars)
	}
	if vars.HasInfra {
		if vars.UseVSO {
			files[overlay+"/vault-auth.yaml"] = render("overlay-vault-auth", vars)
			files[overlay+"/vault-static-secret.yaml"] = render("overlay-vault-static-secret", vars)
		} else {
			files[overlay+"/external-secret.yaml"] = render("overlay-external-secret", vars)
		}
	}
	if vars.IsProd {
		files[overlay+"/hpa.yaml"] = render("overlay-hpa", vars)
	}
	if vars.NeedsNetworkPolicy {
		files[overlay+"/networkpolicy.yaml"] = render("overlay-networkpolicy", vars)
	}

	if len(input.InfraReqs) > 0 {
		infraBase := input.AppName + "/infra/" + input.Environment
		files[infraBase+"/kustomization.yaml"] = render("infra-kustomization", vars)
		for _, req := range input.InfraReqs {
			switch req.ServiceType {
			case "cnpg":
				files[infraBase+"/cnpg-database.yaml"] = render("infra-cnpg-database", vars)
			case "kafka":
				files[infraBase+"/kafka-topics.yaml"] = render("infra-kafka-topics", vars)
			case "rabbitmq":
				files[infraBase+"/rabbitmq.yaml"] = render("infra-rabbitmq", vars)
			case "minio":
				files[infraBase+"/minio-job.yaml"] = render("infra-minio-job", vars)
			case "redis":
				files[infraBase+"/redis-acl-job.yaml"] = render("infra-redis-acl-job", vars)
			}
		}
	}

	return files
}

// splitImage splits "registry/project/name:tag" into (name, tag).
func splitImage(image string) (name, tag string) {
	idx := strings.LastIndex(image, ":")
	if idx < 0 || strings.Contains(image[idx:], "/") {
		return image, "latest"
	}
	return image[:idx], image[idx+1:]
}

// ── Default template content ────────────────────────────────────────────────
// These are Go text/template strings. Admins can replace any of them at runtime
// via the Manifest Templates tab in the admin UI. Use {{.FieldName}} syntax —
// see TemplateVars for all available fields.

const tmplBaseKustomization = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
# Base resources — env-agnostic. Overlays reference ../../base and add:
#   namespace.yaml, ingressroute.yaml, middleware.yaml, patch-resources.yaml (+ hpa.yaml for prod)
resources:
  - serviceaccount.yaml
  - deployment.yaml
  - service.yaml
# Generated by DevPortal — NexBridge Technologies
`

const tmplBaseDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.AppName}}
  labels:
    app: {{.AppName}}
    app.kubernetes.io/managed-by: devportal
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{.AppName}}
  template:
    metadata:
      labels:
        app: {{.AppName}}
    spec:
      serviceAccountName: {{.AppName}}
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: {{.AppName}}
          image: DEVPORTAL_IMAGE_PLACEHOLDER
          imagePullPolicy: Always
          ports:
            - containerPort: {{.Port}}
              name: http
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
            readOnlyRootFilesystem: true
          livenessProbe:
            httpGet:
              path: {{.HealthPath}}
              port: http
            initialDelaySeconds: {{.LivenessDelay}}
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: {{.HealthPath}}
              port: http
            initialDelaySeconds: {{.ReadinessDelay}}
            periodSeconds: 5
{{- if .ExtraEnv}}
          env:
{{- range $k, $v := .ExtraEnv}}
            - name: {{$k}}
              value: "{{$v}}"
{{- end}}
{{- end}}
          envFrom:
            - configMapRef:
                name: {{.AppName}}-config
                optional: true
            - secretRef:
                name: {{.AppName}}-secret
                optional: true
{{- if .HasInfra}}
            - configMapRef:
                name: {{.AppName}}-infra-config
                optional: true
{{- end}}
{{- if .IsFrontend}}
          volumeMounts:
            # nginx-unprivileged needs these three paths writable even though the
            # image itself has no writable root — official pattern for running
            # nginxinc/nginx-unprivileged under readOnlyRootFilesystem: true.
            - name: tmp
              mountPath: /tmp
            - name: nginx-cache
              mountPath: /var/cache/nginx
            - name: nginx-run
              mountPath: /var/run
      volumes:
        - name: tmp
          emptyDir: {}
        - name: nginx-cache
          emptyDir: {}
        - name: nginx-run
          emptyDir: {}
{{- end}}
`

const tmplBaseService = `apiVersion: v1
kind: Service
metadata:
  name: {{.AppName}}
  labels:
    app: {{.AppName}}
    app.kubernetes.io/managed-by: devportal
spec:
  selector:
    app: {{.AppName}}
  ports:
    - port: 80
      targetPort: {{.Port}}
      name: http
  type: ClusterIP
`

const tmplOverlayKustomization = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: {{.Namespace}}
resources:
  - ../../base
  - namespace.yaml
{{- if not .IsWorker}}
  # Traefik CRD routing (default) — swap for gateway.yaml + httproute.yaml to use K8s Gateway API
  - ingressroute.yaml
  - middleware.yaml
  # K8s Gateway API routing — uncomment and remove ingressroute/middleware above to switch
  # - gateway.yaml
  # - httproute.yaml
{{- end}}
{{- if or .HasInfra .HasDeps}}
  - infra-configmap.yaml
{{- end}}
{{- if .HasInfra}}
{{- if .UseVSO}}
  - vault-auth.yaml
  - vault-static-secret.yaml
{{- else}}
  - external-secret.yaml
{{- end}}
{{- end}}
{{- if .IsProd}}
  - hpa.yaml
{{- end}}
{{- if .NeedsNetworkPolicy}}
  - networkpolicy.yaml
{{- end}}
images:
  - name: DEVPORTAL_IMAGE_PLACEHOLDER
    newName: {{.ImageName}}
    newTag: "{{.ImageTag}}"
patches:
  - path: patch-resources.yaml
    target:
      kind: Deployment
      name: {{.AppName}}
`

const tmplOverlayNamespace = `apiVersion: v1
kind: Namespace
metadata:
  name: {{.Namespace}}
  labels:
    app.kubernetes.io/managed-by: devportal
    environment: {{.Environment}}
`

const tmplOverlayIngressRoute = `apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: {{.AppName}}
  labels:
    app: {{.AppName}}
    environment: {{.Environment}}
    app.kubernetes.io/managed-by: devportal
spec:
  entryPoints:
    - websecure
  routes:
    - match: Host(` + "`" + `{{.IngressHost}}` + "`" + `)
      kind: Rule
      middlewares:
        - name: {{.AppName}}-mw
          namespace: {{.Namespace}}
      services:
        - name: {{.AppName}}
          port: 80
  tls:
    secretName: {{.AppName}}-tls
`

const tmplOverlayMiddleware = `apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: {{.AppName}}-mw
  labels:
    app: {{.AppName}}
    environment: {{.Environment}}
    app.kubernetes.io/managed-by: devportal
spec:
  chain:
    middlewares:
      - name: {{.AppName}}-headers
        namespace: {{.Namespace}}
      - name: {{.AppName}}-ratelimit
        namespace: {{.Namespace}}
---
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: {{.AppName}}-headers
  labels:
    app: {{.AppName}}
    app.kubernetes.io/managed-by: devportal
spec:
  headers:
    customResponseHeaders:
      X-Content-Type-Options: nosniff
      X-Frame-Options: DENY
      X-XSS-Protection: "1; mode=block"
      Strict-Transport-Security: "max-age=31536000; includeSubDomains"
---
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: {{.AppName}}-ratelimit
  labels:
    app: {{.AppName}}
    app.kubernetes.io/managed-by: devportal
spec:
  rateLimit:
    average: 100
    burst: 50
`

const tmplOverlayGateway = `apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: {{.AppName}}-gateway
  namespace: {{.Namespace}}
  labels:
    app: {{.AppName}}
    environment: {{.Environment}}
    app.kubernetes.io/managed-by: devportal
spec:
  gatewayClassName: traefik
  listeners:
    - name: websecure
      protocol: HTTPS
      port: 443
      hostname: {{.IngressHost}}
      tls:
        mode: Terminate
        certificateRefs:
          - name: {{.AppName}}-tls
            kind: Secret
            group: ""
      allowedRoutes:
        namespaces:
          from: Same
    - name: web
      protocol: HTTP
      port: 80
      hostname: {{.IngressHost}}
      allowedRoutes:
        namespaces:
          from: Same
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: {{.AppName}}-redirect
  namespace: {{.Namespace}}
  labels:
    app: {{.AppName}}
    environment: {{.Environment}}
    app.kubernetes.io/managed-by: devportal
spec:
  parentRefs:
    - name: {{.AppName}}-gateway
      namespace: {{.Namespace}}
      sectionName: web
  hostnames:
    - {{.IngressHost}}
  rules:
    - filters:
        - type: RequestRedirect
          requestRedirect:
            scheme: https
            statusCode: 301
`

const tmplOverlayHTTPRoute = `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: {{.AppName}}
  namespace: {{.Namespace}}
  labels:
    app: {{.AppName}}
    environment: {{.Environment}}
    app.kubernetes.io/managed-by: devportal
spec:
  parentRefs:
    - name: {{.AppName}}-gateway
      namespace: {{.Namespace}}
      sectionName: websecure
  hostnames:
    - {{.IngressHost}}
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: {{.AppName}}
          port: 80
          weight: 100
`

const tmplOverlayInfraConfigMap = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{.AppName}}-infra-config
  namespace: {{.Namespace}}
  labels:
    app: {{.AppName}}
    environment: {{.Environment}}
    app.kubernetes.io/managed-by: devportal
data:
{{- range .InfraReqs}}
{{- if eq .ServiceType "cnpg"}}
  DB_HOST: {{$.CNPGClusterName}}-rw.{{$.CNPGClusterNamespace}}.svc.cluster.local
  DB_PORT: "5432"
  DB_NAME: {{$.DBName}}
  DB_USER: {{$.DBUser}}
  DB_SSL_MODE: require
{{- end}}
{{- if eq .ServiceType "kafka"}}
  KAFKA_BOOTSTRAP_SERVERS: {{$.KafkaClusterName}}-kafka-bootstrap.{{$.KafkaClusterNamespace}}.svc.cluster.local:9092
  KAFKA_SECURITY_PROTOCOL: PLAINTEXT
{{- end}}
{{- if eq .ServiceType "rabbitmq"}}
  RABBITMQ_HOST: {{$.RabbitMQClusterName}}.{{$.RabbitMQClusterNamespace}}.svc.cluster.local
  RABBITMQ_PORT: "5672"
  RABBITMQ_VHOST: {{$.RabbitVhost}}
{{- end}}
{{- if eq .ServiceType "redis"}}
  REDIS_HOST: redis-master.{{$.RedisNamespace}}.svc.cluster.local
  REDIS_PORT: "6379"
  REDIS_USERNAME: {{$.AppName}}-{{$.Environment}}
{{- end}}
{{- if eq .ServiceType "minio"}}
  MINIO_ENDPOINT: http://minio.{{$.MinIONamespace}}.svc.cluster.local:9000
  MINIO_BUCKET: {{$.AppName}}-{{$.Environment}}
{{- end}}
{{- end}}
{{- range .ResolvedDeps}}
  {{.EnvKey}}: {{.ServiceURL}}
{{- end}}
`

const tmplOverlayExternalSecret = `apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: {{.AppName}}-secret
  namespace: {{.Namespace}}
  labels:
    app: {{.AppName}}
    environment: {{.Environment}}
    app.kubernetes.io/managed-by: devportal
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault-backend
    kind: ClusterSecretStore
  target:
    name: {{.AppName}}-secret
    creationPolicy: Owner
  data:
{{- range .InfraReqs}}
{{- if eq .ServiceType "cnpg"}}
    - secretKey: DB_PASSWORD
      remoteRef:
        key: devportal/{{$.AppName}}/{{$.Environment}}/db
        property: password
{{- end}}
{{- if eq .ServiceType "kafka"}}
    - secretKey: KAFKA_SASL_PASSWORD
      remoteRef:
        key: devportal/{{$.AppName}}/{{$.Environment}}/kafka
        property: password
{{- end}}
{{- if eq .ServiceType "rabbitmq"}}
    - secretKey: RABBITMQ_PASSWORD
      remoteRef:
        key: devportal/{{$.AppName}}/{{$.Environment}}/rabbitmq
        property: password
{{- end}}
{{- if eq .ServiceType "redis"}}
    - secretKey: REDIS_PASSWORD
      remoteRef:
        key: devportal/{{$.AppName}}/{{$.Environment}}/redis
        property: password
{{- end}}
{{- if eq .ServiceType "minio"}}
    - secretKey: MINIO_ACCESS_KEY
      remoteRef:
        key: devportal/{{$.AppName}}/{{$.Environment}}/minio
        property: access_key
    - secretKey: MINIO_SECRET_KEY
      remoteRef:
        key: devportal/{{$.AppName}}/{{$.Environment}}/minio
        property: secret_key
{{- end}}
{{- end}}
`

const tmplBaseServiceAccount = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{.AppName}}
  labels:
    app: {{.AppName}}
    app.kubernetes.io/managed-by: devportal
  annotations: {}
  # Vault agent: vault.hashicorp.com/agent-inject: "true"
  # AWS IRSA:    eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT:role/ROLE"
  # GCP WI:      iam.gke.io/gcp-service-account: "SA@PROJECT.iam.gserviceaccount.com"
`

const tmplOverlayResourcesPatch = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.AppName}}
spec:
  replicas: {{.Replicas}}
  template:
    spec:
      containers:
        - name: {{.AppName}}
          resources:
            requests:
              cpu: {{.CPUReq}}
              memory: {{.MemReq}}
            limits:
              cpu: {{.CPULim}}
              memory: {{.MemLim}}
`

const tmplOverlayHPA = `apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: {{.AppName}}
  namespace: {{.Namespace}}
  labels:
    app: {{.AppName}}
    app.kubernetes.io/managed-by: devportal
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{.AppName}}
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
`

const tmplOverlayVaultAuth = `apiVersion: secrets.hashicorp.com/v1beta1
kind: VaultAuth
metadata:
  name: {{.AppName}}-vault-auth
  namespace: {{.Namespace}}
  labels:
    app: {{.AppName}}
    environment: {{.Environment}}
    app.kubernetes.io/managed-by: devportal
spec:
  method: kubernetes
  mount: {{.VaultK8sMount}}
  kubernetes:
    role: {{.VaultRole}}
    serviceAccount: {{.AppName}}
    audiences:
      - vault
`

const tmplOverlayVaultStaticSecret = `apiVersion: secrets.hashicorp.com/v1beta1
kind: VaultStaticSecret
metadata:
  name: {{.AppName}}-secret
  namespace: {{.Namespace}}
  labels:
    app: {{.AppName}}
    environment: {{.Environment}}
    app.kubernetes.io/managed-by: devportal
spec:
  type: kv-v2
  mount: {{.VaultMount}}
  path: {{.VaultKVPath}}
  destination:
    name: {{.AppName}}-secret
    create: true
  vaultAuthRef: {{.AppName}}-vault-auth
  refreshAfter: 1h
`

const tmplOverlayNetworkPolicy = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{.AppName}}
  labels:
    app: {{.AppName}}
    app.kubernetes.io/managed-by: devportal
spec:
  podSelector:
    matchLabels:
      app: {{.AppName}}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: traefik
      ports:
        - port: {{.Port}}
          protocol: TCP
  egress:
    - ports:
        - port: 53
          protocol: UDP
        - port: 53
          protocol: TCP
{{- range .Deps}}
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: {{.TargetSlug}}-{{$.Environment}}
          podSelector:
            matchLabels:
              app: {{.TargetSlug}}
      ports:
        - port: {{.Port}}
          protocol: TCP
{{- end}}
{{- range .InfraReqs}}
{{- if eq .ServiceType "cnpg"}}
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: {{$.CNPGClusterNamespace}}
      ports:
        - port: 5432
          protocol: TCP
{{- end}}
{{- if eq .ServiceType "kafka"}}
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: {{$.KafkaClusterNamespace}}
      ports:
        - port: 9092
          protocol: TCP
{{- end}}
{{- if eq .ServiceType "rabbitmq"}}
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: {{$.RabbitMQClusterNamespace}}
      ports:
        - port: 5672
          protocol: TCP
{{- end}}
{{- if eq .ServiceType "redis"}}
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: {{$.RedisNamespace}}
      ports:
        - port: 6379
          protocol: TCP
{{- end}}
{{- if eq .ServiceType "minio"}}
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: {{$.MinIONamespace}}
      ports:
        - port: 9000
          protocol: TCP
{{- end}}
{{- end}}
`

const tmplInfraKustomization = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
# Platform operator CRs for {{.AppName}} ({{.Environment}}).
# Managed by a SEPARATE ArgoCD Application watching {{.AppName}}/infra/{{.Environment}}.
# CRs carry their own namespace — not overridden by this kustomization.
resources:
{{- range .InfraReqs}}
{{- if eq .ServiceType "cnpg"}}
  - cnpg-database.yaml
{{- end}}
{{- if eq .ServiceType "kafka"}}
  - kafka-topics.yaml
{{- end}}
{{- if eq .ServiceType "rabbitmq"}}
  - rabbitmq.yaml
{{- end}}
{{- if eq .ServiceType "minio"}}
  - minio-job.yaml
{{- end}}
{{- if eq .ServiceType "redis"}}
  - redis-acl-job.yaml
{{- end}}
{{- end}}
`

const tmplInfraCNPGDatabase = `apiVersion: postgresql.cnpg.io/v1
kind: Database
metadata:
  name: {{.AppName}}-{{.Environment}}
  namespace: {{.CNPGClusterNamespace}}
  labels:
    app: {{.AppName}}
    environment: {{.Environment}}
    app.kubernetes.io/managed-by: devportal
  annotations:
    devportal.nexbridge.io/service: {{.AppName}}
spec:
  name: {{.DBName}}
  owner: {{.DBUser}}
  cluster:
    name: {{.CNPGClusterName}}
`

const tmplInfraKafkaTopics = `{{- range .KafkaTopics}}
---
apiVersion: kafka.strimzi.io/v1beta2
kind: KafkaTopic
metadata:
  name: {{.}}
  namespace: {{$.KafkaClusterNamespace}}
  labels:
    strimzi.io/cluster: {{$.KafkaClusterName}}
    app: {{$.AppName}}
    environment: {{$.Environment}}
    app.kubernetes.io/managed-by: devportal
spec:
  partitions: 3
  replicas: 1
  config:
    retention.ms: "604800000"
    segment.bytes: "1073741824"
{{- end}}
`

const tmplInfraRedisACLJob = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{.AppName}}-{{.Environment}}-redis-acl
  namespace: {{.RedisNamespace}}
  labels:
    app: {{.AppName}}
    environment: {{.Environment}}
    app.kubernetes.io/managed-by: devportal
  annotations:
    # ArgoCD sync hook — re-runs on every sync; safe because ACL SETUSER is idempotent.
    argocd.argoproj.io/hook: Sync
    argocd.argoproj.io/hook-delete-policy: BeforeHookCreation
spec:
  ttlSecondsAfterFinished: 300
  backoffLimit: 3
  template:
    metadata:
      labels:
        app: {{.AppName}}-redis-acl-init
        environment: {{.Environment}}
    spec:
      restartPolicy: OnFailure
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: redis-cli
          image: redis:7-alpine
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
            readOnlyRootFilesystem: true
          command:
            - /bin/sh
            - -c
            - |
              # reset — wipe any prior rules for this user (idempotent re-runs)
              # on nopass — enable without requiring a password (NetworkPolicy provides L4 isolation)
              # ~<prefix>:* — restrict key access to this service's namespace only
              # &* — allow all pub/sub channels
              # +@all — allow all commands (tighten per security policy if needed)
              redis-cli \
                -h redis-master.{{.RedisNamespace}}.svc.cluster.local \
                -p 6379 \
                -a "$REDIS_ADMIN_PASSWORD" \
                ACL SETUSER "{{.AppName}}-{{.Environment}}" \
                  reset on nopass \
                  "~{{.AppName}}-{{.Environment}}:*" \
                  "&*" "+@all"
              redis-cli \
                -h redis-master.{{.RedisNamespace}}.svc.cluster.local \
                -p 6379 \
                -a "$REDIS_ADMIN_PASSWORD" \
                ACL SAVE
              echo "ACL user {{.AppName}}-{{.Environment}} ready"
          env:
            - name: REDISCLI_HISTFILE
              value: /dev/null
            - name: REDIS_ADMIN_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{.RedisAdminSecret}}
                  key: redis-password
`

const tmplInfraMinIOJob = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{.AppName}}-{{.Environment}}-create-bucket
  namespace: {{.MinIONamespace}}
  labels:
    app: {{.AppName}}
    environment: {{.Environment}}
    app.kubernetes.io/managed-by: devportal
  annotations:
    # ArgoCD sync hook — re-runs on every sync; safe because mc mb is idempotent.
    argocd.argoproj.io/hook: Sync
    argocd.argoproj.io/hook-delete-policy: BeforeHookCreation
spec:
  ttlSecondsAfterFinished: 300
  backoffLimit: 3
  template:
    metadata:
      labels:
        app: {{.AppName}}-bucket-init
        environment: {{.Environment}}
    spec:
      restartPolicy: OnFailure
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: mc
          # Pin to a recent stable release. Override via the admin manifest template editor.
          image: minio/mc:RELEASE.2025-04-16T15-43-50Z
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
            readOnlyRootFilesystem: true
          command:
            - /bin/sh
            - -c
            - |
              mc alias set local \
                http://minio.{{.MinIONamespace}}.svc.cluster.local:9000 \
                "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"
              mc mb --ignore-existing "local/{{.AppName}}-{{.Environment}}"
              mc anonymous set none "local/{{.AppName}}-{{.Environment}}"
              echo "bucket {{.AppName}}-{{.Environment}} ready"
          env:
            - name: MC_CONFIG_DIR
              value: /tmp/mc-config
            - name: MINIO_ROOT_USER
              valueFrom:
                secretKeyRef:
                  name: {{.MinIOAdminSecret}}
                  key: rootUser
            - name: MINIO_ROOT_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{.MinIOAdminSecret}}
                  key: rootPassword
          volumeMounts:
            - name: mc-config
              mountPath: /tmp/mc-config
      volumes:
        - name: mc-config
          emptyDir: {}
`

const tmplInfraRabbitMQ = `apiVersion: rabbitmq.com/v1beta1
kind: Vhost
metadata:
  name: {{.AppName}}-{{.Environment}}
  namespace: {{.RabbitMQClusterNamespace}}
  labels:
    app: {{.AppName}}
    environment: {{.Environment}}
    app.kubernetes.io/managed-by: devportal
spec:
  name: {{.RabbitVhost}}
  rabbitmqClusterReference:
    name: {{.RabbitMQClusterName}}
    namespace: {{.RabbitMQClusterNamespace}}
---
apiVersion: rabbitmq.com/v1beta1
kind: User
metadata:
  name: {{.AppName}}-{{.Environment}}
  namespace: {{.RabbitMQClusterNamespace}}
  labels:
    app: {{.AppName}}
    environment: {{.Environment}}
    app.kubernetes.io/managed-by: devportal
spec:
  tags:
    - management
  rabbitmqClusterReference:
    name: {{.RabbitMQClusterName}}
    namespace: {{.RabbitMQClusterNamespace}}
  userSecretReference:
    name: {{.AppName}}-{{.Environment}}-rabbit-creds
    namespace: {{.RabbitMQClusterNamespace}}
`
