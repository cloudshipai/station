package targets

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"station/internal/deployment"
)

type KubernetesTarget struct{}

func NewKubernetesTarget() *KubernetesTarget {
	return &KubernetesTarget{}
}

func (k *KubernetesTarget) Name() string {
	return "kubernetes"
}

func (k *KubernetesTarget) Validate(ctx context.Context) error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl not found: install from https://kubernetes.io/docs/tasks/tools/")
	}
	return nil
}

func (k *KubernetesTarget) GenerateConfig(ctx context.Context, config *deployment.DeploymentConfig, secrets map[string]string) (map[string]string, error) {
	return k.GenerateConfigWithOptions(ctx, config, secrets, deployment.DeployOptions{})
}

func (k *KubernetesTarget) GenerateConfigWithOptions(ctx context.Context, config *deployment.DeploymentConfig, secrets map[string]string, options deployment.DeployOptions) (map[string]string, error) {
	files := make(map[string]string)

	namespace := "default"
	if config.Namespace != "" {
		namespace = config.Namespace
	}

	appName := fmt.Sprintf("station-%s", config.EnvironmentName)

	files["namespace.yaml"] = k.generateNamespace(namespace)
	files["secret.yaml"] = k.generateSecret(appName, namespace, secrets)
	files["deployment.yaml"] = k.generateDeployment(appName, namespace, config, secrets, options)
	files["service.yaml"] = k.generateService(appName, namespace, config)
	files["ingress.yaml"] = k.generateIngress(appName, namespace, config)
	files["pvc.yaml"] = k.generatePVC(appName, namespace)
	files["kustomization.yaml"] = k.generateKustomization(appName, options)

	if options.BundlePath != "" {
		bundleConfigMap, err := k.generateBundleConfigMap(appName, namespace, options.BundlePath)
		if err != nil {
			return nil, fmt.Errorf("failed to generate bundle ConfigMap: %w", err)
		}
		files["bundle-configmap.yaml"] = bundleConfigMap
	}

	return files, nil
}

func (k *KubernetesTarget) Deploy(ctx context.Context, config *deployment.DeploymentConfig, secrets map[string]string, options deployment.DeployOptions) error {
	files, err := k.GenerateConfigWithOptions(ctx, config, secrets, options)
	if err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	outputDir := options.OutputDir
	if outputDir == "" {
		outputDir = fmt.Sprintf("k8s-%s", config.EnvironmentName)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for filename, content := range files {
		path := fmt.Sprintf("%s/%s", outputDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
		fmt.Printf("   ✓ Generated %s\n", path)
	}

	if options.DryRun {
		fmt.Printf("\n📄 Dry run - files generated in %s/\n", outputDir)
		fmt.Printf("   To apply: kubectl apply -k %s/\n", outputDir)
		return nil
	}

	fmt.Printf("\n🚀 Applying Kubernetes manifests...\n")

	args := []string{"apply", "-k", outputDir}
	if options.Context != "" {
		args = append([]string{"--context", options.Context}, args...)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl apply failed: %w", err)
	}

	fmt.Printf("\n✅ Deployment complete!\n")
	return nil
}

func (k *KubernetesTarget) Destroy(ctx context.Context, config *deployment.DeploymentConfig) error {
	outputDir := fmt.Sprintf("k8s-%s", config.EnvironmentName)

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return fmt.Errorf("no deployment found at %s", outputDir)
	}

	fmt.Printf("🗑️  Destroying Kubernetes deployment...\n")

	cmd := exec.CommandContext(ctx, "kubectl", "delete", "-k", outputDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl delete failed: %w", err)
	}

	fmt.Printf("✅ Deployment destroyed\n")
	return nil
}

func (k *KubernetesTarget) Status(ctx context.Context, config *deployment.DeploymentConfig) (*deployment.DeploymentStatus, error) {
	appName := fmt.Sprintf("station-%s", config.EnvironmentName)
	namespace := config.Namespace
	if namespace == "" {
		namespace = "default"
	}

	cmd := exec.CommandContext(ctx, "kubectl", "get", "deployment", appName,
		"-n", namespace, "-o", "jsonpath={.status.readyReplicas}/{.status.replicas}")
	output, err := cmd.Output()
	if err != nil {
		return &deployment.DeploymentStatus{State: "unknown", Message: err.Error()}, nil
	}

	parts := strings.Split(string(output), "/")
	status := &deployment.DeploymentStatus{
		State:    "running",
		Metadata: make(map[string]string),
	}

	if len(parts) == 2 {
		status.Message = fmt.Sprintf("%s/%s replicas ready", parts[0], parts[1])
		if parts[0] == "0" {
			status.State = "pending"
		}
	}

	svcCmd := exec.CommandContext(ctx, "kubectl", "get", "svc", appName,
		"-n", namespace, "-o", "jsonpath={.status.loadBalancer.ingress[0].ip}")
	if svcOutput, err := svcCmd.Output(); err == nil && len(svcOutput) > 0 {
		status.Endpoints = append(status.Endpoints, fmt.Sprintf("http://%s:%s", string(svcOutput), config.MCPPort))
	}

	return status, nil
}

func (k *KubernetesTarget) generateNamespace(namespace string) string {
	if namespace == "default" {
		return ""
	}
	return fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
`, namespace)
}

func (k *KubernetesTarget) generateSecret(appName, namespace string, secrets map[string]string) string {
	var secretData strings.Builder
	for key, value := range secrets {
		secretData.WriteString(fmt.Sprintf("  %s: %q\n", key, value))
	}

	return fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s-secrets
  namespace: %s
type: Opaque
stringData:
%s`, appName, namespace, secretData.String())
}

func (k *KubernetesTarget) generateDeployment(appName, namespace string, config *deployment.DeploymentConfig, secrets map[string]string, options deployment.DeployOptions) string {
	var envVars strings.Builder

	for key := range secrets {
		envVars.WriteString(fmt.Sprintf(`        - name: %s
          valueFrom:
            secretKeyRef:
              name: %s-secrets
              key: %s
`, key, appName, key))
	}

	cpu := "500m"
	memory := "512Mi"
	if config.ResourceSize == "medium" {
		cpu = "1000m"
		memory = "1Gi"
	} else if config.ResourceSize == "large" {
		cpu = "2000m"
		memory = "2Gi"
	}

	replicas := 1
	if config.Replicas > 0 {
		replicas = config.Replicas
	}

	bundleVolumeMount := ""
	bundleVolume := ""
	commandOverride := ""

	if options.BundlePath != "" {
		bundleVolumeMount = `        - name: bundle-source
          mountPath: /bundle
          readOnly: true
`
		bundleVolume = fmt.Sprintf(`      - name: bundle-source
        configMap:
          name: %s-bundle
`, appName)
		commandOverride = `        command: ["/bin/sh", "-c"]
        args:
        - |
          stn init && \
          stn bundle install /bundle/bundle.tar.gz default --force && \
          stn sync default -i=false && \
          stn serve
`
	}

	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      containers:
      - name: station
        image: %s
        imagePullPolicy: IfNotPresent
%s        ports:
        - name: mcp
          containerPort: 8586
        - name: dynamic-mcp
          containerPort: 8587
        env:
%s
        resources:
          requests:
            cpu: %s
            memory: %s
          limits:
            cpu: %s
            memory: %s
        volumeMounts:
        - name: data
          mountPath: /home/station/.config/station
%s        livenessProbe:
          httpGet:
            path: /health
            port: 8587
          initialDelaySeconds: 60
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8587
          initialDelaySeconds: 10
          periodSeconds: 5
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: %s-data
%s`, appName, namespace, appName, replicas, appName, appName, config.DockerImage, commandOverride, envVars.String(), cpu, memory, cpu, memory, bundleVolumeMount, appName, bundleVolume)
}

func (k *KubernetesTarget) generateService(appName, namespace string, config *deployment.DeploymentConfig) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
spec:
  type: ClusterIP
  selector:
    app: %s
  ports:
  - name: mcp
    port: 8586
    targetPort: 8586
  - name: dynamic-mcp
    port: 8587
    targetPort: 8587
`, appName, namespace, appName)
}

func (k *KubernetesTarget) generateIngress(appName, namespace string, config *deployment.DeploymentConfig) string {
	return fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %s
  namespace: %s
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "50m"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
spec:
  ingressClassName: nginx
  rules:
  - host: %s.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: %s
            port:
              number: 8587
      - path: /mcp
        pathType: Prefix
        backend:
          service:
            name: %s
            port:
              number: 8586
`, appName, namespace, appName, appName, appName)
}

func (k *KubernetesTarget) generatePVC(appName, namespace string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: %s-data
  namespace: %s
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
`, appName, namespace)
}

func (k *KubernetesTarget) generateKustomization(appName string, options deployment.DeployOptions) string {
	resources := `- namespace.yaml
- secret.yaml
- pvc.yaml
- deployment.yaml
- service.yaml
- ingress.yaml`

	if options.BundlePath != "" {
		resources += `
- bundle-configmap.yaml`
	}

	return fmt.Sprintf(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
%s

commonLabels:
  app.kubernetes.io/name: %s
  app.kubernetes.io/managed-by: station-cli
`, resources, appName)
}

func (k *KubernetesTarget) generateBundleConfigMap(appName, namespace, bundlePath string) (string, error) {
	bundleData, err := os.ReadFile(bundlePath)
	if err != nil {
		return "", fmt.Errorf("failed to read bundle file: %w", err)
	}

	bundleBase64 := base64.StdEncoding.EncodeToString(bundleData)
	bundleFilename := filepath.Base(bundlePath)

	return fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s-bundle
  namespace: %s
binaryData:
  bundle.tar.gz: %s
  original-filename: %s
`, appName, namespace, bundleBase64, base64.StdEncoding.EncodeToString([]byte(bundleFilename))), nil
}

func init() {
	deployment.RegisterDeploymentTarget(NewKubernetesTarget())
}
