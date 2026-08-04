// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// manifest.go generates Kubernetes manifests for a project environment.
// Each environment (dev / uat / prod) gets its own set of YAML files
// committed to the manifest repository for ArgoCD to sync.

package template

import "fmt"

// ManifestInput holds the per-environment values needed to render K8s manifests.
type ManifestInput struct {
	AppName     string // e.g. "payment-service"
	Namespace   string // K8s namespace, e.g. "payment-service-dev"
	Environment string // "dev" | "uat" | "prod"
	Image       string // full image ref, e.g. "registry.example.com/backend/payment-service:latest"
	IngressHost string // e.g. "payment-service-dev.apps.example.com"
}

// ManifestSet is the rendered collection of YAML files for one environment.
type ManifestSet struct {
	Namespace  string // namespace.yaml content
	Deployment string // deployment.yaml content
	Service    string // service.yaml content
	Ingress    string // ingress.yaml content
	HPA        string // hpa.yaml content (prod only; empty for dev/uat)
}

// Manifests renders all K8s YAML files for a single environment.
func (g *Generator) Manifests(input ManifestInput) ManifestSet {
	replicas, cpu, mem, cpuLim, memLim := resourceProfile(input.Environment)
	return ManifestSet{
		Namespace:  renderNamespace(input),
		Deployment: renderDeployment(input, replicas, cpu, mem, cpuLim, memLim),
		Service:    renderService(input),
		Ingress:    renderIngress(input),
		HPA:        renderHPA(input),
	}
}

// resourceProfile returns resource requests/limits tuned per environment.
func resourceProfile(env string) (replicas int, cpuReq, memReq, cpuLim, memLim string) {
	switch env {
	case "prod":
		return 2, "200m", "256Mi", "1000m", "1Gi"
	case "uat":
		return 1, "100m", "128Mi", "500m", "512Mi"
	default: // dev
		return 1, "50m", "64Mi", "200m", "256Mi"
	}
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

func renderDeployment(i ManifestInput, replicas int, cpuReq, memReq, cpuLim, memLim string) string {
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    environment: %s
    app.kubernetes.io/managed-by: devportal
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
        environment: %s
    spec:
      containers:
        - name: %s
          image: %s
          imagePullPolicy: Always
          ports:
            - containerPort: 8080
              name: http
          resources:
            requests:
              cpu: %s
              memory: %s
            limits:
              cpu: %s
              memory: %s
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 10
            periodSeconds: 5
          envFrom:
            - configMapRef:
                name: %s-config
            - secretRef:
                name: %s-secret
                optional: true
`,
		i.AppName, i.Namespace,
		i.AppName, i.Environment,
		replicas,
		i.AppName,
		i.AppName, i.Environment,
		i.AppName, i.Image,
		cpuReq, memReq, cpuLim, memLim,
		i.AppName, i.AppName,
	)
}

func renderService(i ManifestInput) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    environment: %s
    app.kubernetes.io/managed-by: devportal
spec:
  selector:
    app: %s
  ports:
    - port: 80
      targetPort: 8080
      name: http
  type: ClusterIP
`, i.AppName, i.Namespace, i.AppName, i.Environment, i.AppName)
}

func renderIngress(i ManifestInput) string {
	return fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    environment: %s
    app.kubernetes.io/managed-by: devportal
  annotations:
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    traefik.ingress.kubernetes.io/router.tls: "true"
spec:
  rules:
    - host: %s
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: %s
                port:
                  name: http
  tls:
    - hosts:
        - %s
      secretName: %s-tls
`, i.AppName, i.Namespace, i.AppName, i.Environment, i.IngressHost, i.AppName, i.IngressHost, i.AppName)
}

// renderHPA generates a HorizontalPodAutoscaler for prod; returns empty for dev/uat.
func renderHPA(i ManifestInput) string {
	if i.Environment != "prod" {
		return ""
	}
	return fmt.Sprintf(`apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    app.kubernetes.io/managed-by: devportal
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: %s
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
`, i.AppName, i.Namespace, i.AppName, i.AppName)
}
