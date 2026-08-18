// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// manifest.go generates a Kustomize base+overlays file tree for a provisioned service.
// The orchestrator iterates the returned KustomizeFiles map and commits each
// path→content pair to the manifest repository.
//
// Repository layout produced (one branch per environment):
//
//	<service>/base/kustomization.yaml            — lists base resources
//	<service>/base/deployment.yaml               — env-agnostic Deployment (DEVPORTAL_IMAGE_PLACEHOLDER)
//	<service>/base/service.yaml                  — ClusterIP Service
//	<service>/overlays/<env>/kustomization.yaml  — sets namespace, resolves image, applies patches
//	<service>/overlays/<env>/namespace.yaml      — creates the Namespace object
//	<service>/overlays/<env>/ingress.yaml        — Ingress with env-specific host
//	<service>/overlays/<env>/patch-resources.yaml — replicas + CPU/mem per env tier
//	<service>/overlays/<env>/hpa.yaml            — HPA (prod only)
//	<service>/overlays/<env>/networkpolicy.yaml  — NetworkPolicy (when service deps or infra reqs exist)
//	<service>/infra/<env>/kustomization.yaml     — operator CRs (separate ArgoCD Application)
//	<service>/infra/<env>/cnpg-database.yaml     — CNPG Database CR (if cnpg selected)
//	<service>/infra/<env>/kafka-topics.yaml      — Strimzi KafkaTopic CRs (if kafka selected)
//	<service>/infra/<env>/rabbitmq.yaml          — RabbitMQ operator CRs (if rabbitmq selected)
//
// Jenkins updates the image tag by running:
//
//	kustomize edit set image DEVPORTAL_IMAGE_PLACEHOLDER=<registry>/<project>/<service>:<build>
//
// inside the relevant overlay directory, then committing the change.
package template

import (
	"fmt"
	"strings"
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
	MinIONamespace           string
}

// ResourceSpec carries the environment-tier resource values read from the
// environment_profiles DB table. Replaces the old hardcoded resourceProfile() function.
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
	Resources   ResourceSpec
	Lang        LangProfile
	InfraReqs   []InfraRequirement
	Deps        []ServiceDep
	Platform    PlatformRefs
}

// KustomizeFiles maps repository-relative file path → YAML content.
// Every entry is committed to the manifest repo by the orchestrator.
type KustomizeFiles map[string]string

// KustomizeManifests returns the complete Kustomize file tree for one environment.
// base/ files are identical across all environments — committing them on each
// branch is idempotent and ensures ArgoCD always has a consistent base to reference.
func (g *Generator) KustomizeManifests(input ManifestInput) KustomizeFiles {
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
	imageName, imageTag := splitImage(input.Image)

	base    := input.AppName + "/base"
	overlay := input.AppName + "/overlays/" + input.Environment

	// Determine extra resources for the overlay kustomization.yaml
	needNetworkPolicy := len(input.Deps) > 0 || len(input.InfraReqs) > 0
	var overlayExtras []string
	if input.Environment == "prod" {
		overlayExtras = append(overlayExtras, "hpa.yaml")
	}
	if needNetworkPolicy {
		overlayExtras = append(overlayExtras, "networkpolicy.yaml")
	}

	files := KustomizeFiles{
		// ── Base — env-agnostic, identical on every branch ──────────────────
		base + "/kustomization.yaml": renderBaseKustomization(),
		base + "/deployment.yaml":    renderBaseDeployment(input.AppName, port, healthPath, lang),
		base + "/service.yaml":       renderBaseService(input.AppName, port),

		// ── Overlay — env-specific ───────────────────────────────────────────
		overlay + "/kustomization.yaml":   renderOverlayKustomization(input, imageName, imageTag, overlayExtras),
		overlay + "/namespace.yaml":       renderNamespace(input),
		overlay + "/ingress.yaml":         renderIngress(input),
		overlay + "/patch-resources.yaml": renderResourcePatch(input.AppName, res),
	}

	if input.Environment == "prod" {
		files[overlay+"/hpa.yaml"] = renderHPA(input.AppName, input.Namespace)
	}
	if needNetworkPolicy {
		files[overlay+"/networkpolicy.yaml"] = renderNetworkPolicy(input, port)
	}

	// Generate platform operator CRs in infra/<env>/ (managed by a separate ArgoCD Application)
	if len(input.InfraReqs) > 0 {
		infraBase := input.AppName + "/infra/" + input.Environment
		crFiles, infraKustomization := renderInfraFiles(input)
		files[infraBase+"/kustomization.yaml"] = infraKustomization
		for name, content := range crFiles {
			files[infraBase+"/"+name] = content
		}
	}

	return files
}

// splitImage splits "registry/project/name:tag" into (name, tag).
// Handles the edge case where the tag contains a slash (unlikely but safe).
func splitImage(image string) (name, tag string) {
	idx := strings.LastIndex(image, ":")
	if idx < 0 || strings.Contains(image[idx:], "/") {
		return image, "latest"
	}
	return image[:idx], image[idx+1:]
}

// ── Base renderers ─────────────────────────────────────────────────────────────

func renderBaseKustomization() string {
	return `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
# Base resources — env-agnostic. Overlays reference ../../base and add:
#   namespace.yaml, ingress.yaml, patch-resources.yaml (+ hpa.yaml for prod)
resources:
  - deployment.yaml
  - service.yaml
# Generated by DevPortal — NexBridge Technologies
`
}

func renderBaseDeployment(appName string, port int, healthPath string, lang LangProfile) string {
	extraEnvBlock := ""
	if len(lang.ExtraEnv) > 0 {
		extraEnvBlock = "          env:\n"
		for k, v := range lang.ExtraEnv {
			extraEnvBlock += fmt.Sprintf("            - name: %s\n              value: %q\n", k, v)
		}
	}
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %[1]s
  labels:
    app: %[1]s
    app.kubernetes.io/managed-by: devportal
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %[1]s
  template:
    metadata:
      labels:
        app: %[1]s
    spec:
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: %[1]s
          image: DEVPORTAL_IMAGE_PLACEHOLDER
          imagePullPolicy: Always
          ports:
            - containerPort: %[2]d
              name: http
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
            readOnlyRootFilesystem: true
          livenessProbe:
            httpGet:
              path: %[3]s
              port: http
            initialDelaySeconds: %[4]d
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: %[3]s
              port: http
            initialDelaySeconds: %[5]d
            periodSeconds: 5
%[6]s          envFrom:
            - configMapRef:
                name: %[1]s-config
                optional: true
            - secretRef:
                name: %[1]s-secret
                optional: true
`, appName, port, healthPath, lang.LivenessDelay, lang.ReadinessDelay, extraEnvBlock)
}

func renderBaseService(appName string, port int) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %[1]s
  labels:
    app: %[1]s
    app.kubernetes.io/managed-by: devportal
spec:
  selector:
    app: %[1]s
  ports:
    - port: 80
      targetPort: %[2]d
      name: http
  type: ClusterIP
`, appName, port)
}

// ── Overlay renderers ──────────────────────────────────────────────────────────

func renderOverlayKustomization(i ManifestInput, imageName, imageTag string, extraResources []string) string {
	resourceLines := "  - ../../base\n  - namespace.yaml\n  - ingress.yaml\n"
	for _, r := range extraResources {
		resourceLines += "  - " + r + "\n"
	}
	return fmt.Sprintf(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: %s
resources:
%simages:
  - name: DEVPORTAL_IMAGE_PLACEHOLDER
    newName: %s
    newTag: %s
patches:
  - path: patch-resources.yaml
    target:
      kind: Deployment
      name: %s
`, i.Namespace, resourceLines, imageName, imageTag, i.AppName)
}

func renderNamespace(i ManifestInput) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
  labels:
    app.kubernetes.io/managed-by: devportal
    environment: %s
`, i.Namespace, i.Environment)
}

func renderIngress(i ManifestInput) string {
	return fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %[1]s
  labels:
    app: %[1]s
    environment: %[2]s
    app.kubernetes.io/managed-by: devportal
  annotations:
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    traefik.ingress.kubernetes.io/router.tls: "true"
spec:
  rules:
    - host: %[3]s
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: %[1]s
                port:
                  name: http
  tls:
    - hosts:
        - %[3]s
      secretName: %[1]s-tls
`, i.AppName, i.Environment, i.IngressHost)
}

func renderResourcePatch(appName string, res ResourceSpec) string {
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
spec:
  replicas: %d
  template:
    spec:
      containers:
        - name: %s
          resources:
            requests:
              cpu: %s
              memory: %s
            limits:
              cpu: %s
              memory: %s
`, appName, res.Replicas, appName, res.CPUReq, res.MemReq, res.CPULim, res.MemLim)
}

func renderHPA(appName, namespace string) string {
	return fmt.Sprintf(`apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: %[1]s
  namespace: %[2]s
  labels:
    app: %[1]s
    app.kubernetes.io/managed-by: devportal
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: %[1]s
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
`, appName, namespace)
}

// renderNetworkPolicy generates a NetworkPolicy restricting ingress to the Traefik
// ingress controller and egress to declared service dependencies and infra namespaces.
func renderNetworkPolicy(i ManifestInput, port int) string {
	var egressRules strings.Builder

	// Egress: declared service-to-service dependencies
	for _, dep := range i.Deps {
		egressRules.WriteString(fmt.Sprintf(`    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: %[1]s-%[2]s
          podSelector:
            matchLabels:
              app: %[1]s
      ports:
        - port: %[3]d
          protocol: TCP
`, dep.TargetSlug, i.Environment, dep.Port))
	}

	// Egress: infra namespace rules
	for _, req := range i.InfraReqs {
		switch req.ServiceType {
		case "cnpg":
			egressRules.WriteString(fmt.Sprintf(`    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: %s
      ports:
        - port: 5432
          protocol: TCP
`, i.Platform.CNPGClusterNamespace))
		case "kafka":
			egressRules.WriteString(fmt.Sprintf(`    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: %s
      ports:
        - port: 9092
          protocol: TCP
`, i.Platform.KafkaClusterNamespace))
		case "rabbitmq":
			egressRules.WriteString(fmt.Sprintf(`    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: %s
      ports:
        - port: 5672
          protocol: TCP
`, i.Platform.RabbitMQClusterNamespace))
		case "redis":
			egressRules.WriteString(fmt.Sprintf(`    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: %s
      ports:
        - port: 6379
          protocol: TCP
`, i.Platform.RedisNamespace))
		case "minio":
			egressRules.WriteString(fmt.Sprintf(`    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: %s
      ports:
        - port: 9000
          protocol: TCP
`, i.Platform.MinIONamespace))
		}
	}

	return fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: %[1]s
  labels:
    app: %[1]s
    app.kubernetes.io/managed-by: devportal
spec:
  podSelector:
    matchLabels:
      app: %[1]s
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: traefik
      ports:
        - port: %[2]d
          protocol: TCP
  egress:
    - ports:
        - port: 53
          protocol: UDP
        - port: 53
          protocol: TCP
%[3]s`, i.AppName, port, egressRules.String())
}

// ── Infra CR renderers ─────────────────────────────────────────────────────────

// renderInfraFiles produces operator CR YAML files and a kustomization.yaml for
// the service's infra requirements. The files live in <service>/infra/<env>/ and
// are managed by a separate ArgoCD Application so their namespaces are not
// overridden by the service overlay's namespace field.
func renderInfraFiles(i ManifestInput) (files map[string]string, kustomization string) {
	files = make(map[string]string)
	var resourceNames []string

	dbName := strings.ReplaceAll(i.AppName, "-", "_") + "_" + i.Environment
	dbUser := dbName + "_user"

	for _, req := range i.InfraReqs {
		switch req.ServiceType {
		case "cnpg":
			files["cnpg-database.yaml"] = renderCNPGDatabase(i, dbName, dbUser)
			resourceNames = append(resourceNames, "cnpg-database.yaml")
		case "kafka":
			files["kafka-topics.yaml"] = renderKafkaTopics(i, req.Config)
			resourceNames = append(resourceNames, "kafka-topics.yaml")
		case "rabbitmq":
			files["rabbitmq.yaml"] = renderRabbitMQResources(i, req.Config)
			resourceNames = append(resourceNames, "rabbitmq.yaml")
		}
	}

	var resourceList strings.Builder
	for _, name := range resourceNames {
		resourceList.WriteString("  - " + name + "\n")
	}

	kustomization = fmt.Sprintf(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
# Platform operator CRs for %[1]s (%[2]s).
# Managed by a SEPARATE ArgoCD Application watching %[1]s/infra/%[2]s on the %[2]s branch.
# CRs carry their own namespace — not overridden by this kustomization.
resources:
%[3]s`, i.AppName, i.Environment, resourceList.String())

	return files, kustomization
}

func renderCNPGDatabase(i ManifestInput, dbName, dbUser string) string {
	return fmt.Sprintf(`apiVersion: postgresql.cnpg.io/v1
kind: Database
metadata:
  name: %[1]s-%[3]s
  namespace: %[4]s
  labels:
    app: %[1]s
    environment: %[3]s
    app.kubernetes.io/managed-by: devportal
  annotations:
    devportal.nexbridge.io/service: %[1]s
spec:
  name: %[2]s
  owner: %[5]s
  cluster:
    name: %[6]s
`, i.AppName, dbName, i.Environment, i.Platform.CNPGClusterNamespace, dbUser, i.Platform.CNPGClusterName)
}

func renderKafkaTopics(i ManifestInput, cfg map[string]string) string {
	topicsRaw := cfg["topics"]
	if topicsRaw == "" {
		topicsRaw = i.AppName + "-events"
	}

	var sb strings.Builder
	for _, name := range strings.Split(topicsRaw, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf(`---
apiVersion: kafka.strimzi.io/v1beta2
kind: KafkaTopic
metadata:
  name: %[1]s
  namespace: %[2]s
  labels:
    strimzi.io/cluster: %[3]s
    app: %[4]s
    environment: %[5]s
    app.kubernetes.io/managed-by: devportal
spec:
  partitions: 3
  replicas: 1
  config:
    retention.ms: "604800000"
    segment.bytes: "1073741824"
`, name, i.Platform.KafkaClusterNamespace, i.Platform.KafkaClusterName, i.AppName, i.Environment))
	}
	return sb.String()
}

func renderRabbitMQResources(i ManifestInput, cfg map[string]string) string {
	vhost := cfg["vhost"]
	if vhost == "" {
		vhost = "/" + i.AppName
	}
	resourceName := i.AppName + "-" + i.Environment
	rmqNs := i.Platform.RabbitMQClusterNamespace
	rmqCluster := i.Platform.RabbitMQClusterName

	return fmt.Sprintf(`apiVersion: rabbitmq.com/v1beta1
kind: Vhost
metadata:
  name: %[1]s
  namespace: %[2]s
  labels:
    app: %[3]s
    environment: %[4]s
    app.kubernetes.io/managed-by: devportal
spec:
  name: %[5]s
  rabbitmqClusterReference:
    name: %[6]s
    namespace: %[2]s
---
apiVersion: rabbitmq.com/v1beta1
kind: User
metadata:
  name: %[1]s
  namespace: %[2]s
  labels:
    app: %[3]s
    environment: %[4]s
    app.kubernetes.io/managed-by: devportal
spec:
  tags:
    - management
  rabbitmqClusterReference:
    name: %[6]s
    namespace: %[2]s
  userSecretReference:
    name: %[1]s-rabbit-creds
    namespace: %[2]s
`, resourceName, rmqNs, i.AppName, i.Environment, vhost, rmqCluster)
}
