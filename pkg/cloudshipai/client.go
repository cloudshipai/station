package cloudshipai

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	pb "station/proto/lighthouse"

	"github.com/spf13/viper"
)

type Client struct {
	conn         *grpc.ClientConn
	client       pb.StationServiceClient
	stationID    string
	registrationKey string
	endpoint     string
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	connected    bool
	mu           sync.RWMutex
}

func NewClient() *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (c *Client) Start() error {
	if !viper.GetBool("cloudshipai.enabled") {
		return nil
	}

	c.registrationKey = viper.GetString("cloudshipai.registration_key")
	c.endpoint = viper.GetString("cloudshipai.endpoint")
	
	if c.registrationKey == "" {
		return fmt.Errorf("CloudShip AI registration key not configured")
	}

	log.Printf("Starting CloudShip AI client connection to %s", c.endpoint)

	if err := c.connect(); err != nil {
		return fmt.Errorf("failed to connect to CloudShip AI: %w", err)
	}

	if err := c.register(); err != nil {
		c.disconnect()
		return fmt.Errorf("failed to register with CloudShip AI: %w", err)
	}

	c.wg.Add(1)
	go c.heartbeatLoop()

	log.Printf("CloudShip AI client started successfully")
	return nil
}

func (c *Client) connect() error {
	var opts []grpc.DialOption
	var dialTarget string
	
	// Parse endpoint to handle HTTPS URLs
	if strings.HasPrefix(c.endpoint, "https://") {
		parsedURL, err := url.Parse(c.endpoint)
		if err != nil {
			return fmt.Errorf("invalid endpoint URL: %w", err)
		}
		
		// Use hostname for gRPC dial target, default to port 443 for HTTPS
		dialTarget = parsedURL.Host
		if !strings.Contains(dialTarget, ":") {
			dialTarget += ":443"
		}
		
		// Use TLS for HTTPS endpoints
		config := &tls.Config{
			ServerName: parsedURL.Hostname(),
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(config)))
	} else if c.endpoint == "localhost:5000" || strings.HasPrefix(c.endpoint, "127.0.0.1:") || strings.HasPrefix(c.endpoint, "localhost:") {
		// Local testing endpoints - insecure
		dialTarget = c.endpoint
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// Legacy endpoint format (host:port) - assume secure
		dialTarget = c.endpoint
		hostname := strings.Split(c.endpoint, ":")[0]
		config := &tls.Config{
			ServerName: hostname,
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(config)))
	}

	conn, err := grpc.Dial(dialTarget, opts...)
	if err != nil {
		return err
	}

	c.conn = conn
	c.client = pb.NewStationServiceClient(conn)
	return nil
}

func (c *Client) register() error {
	hostname, _ := os.Hostname()
	
	// Detect deployment type and get appropriate hardware info
	hardwareInfo := detectDeploymentType()
	
	req := &pb.RegisterStationRequest{
		RegistrationKey: c.registrationKey,
		Hostname:        hostname,
		IpAddress:       getLocalIP(),
		Port:            8080, // Station API port
		Version:         "v1.0.0",
		OsInfo:          fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH),
		HardwareInfo:    hardwareInfo,
	}

	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	resp, err := c.client.RegisterStation(ctx, req)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("registration failed: %s", resp.Message)
	}

	c.mu.Lock()
	c.stationID = resp.StationId
	c.connected = true
	c.mu.Unlock()

	log.Printf("Successfully registered with CloudShip AI, Station ID: %s", c.stationID)
	return nil
}

func (c *Client) heartbeatLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.sendHeartbeat(); err != nil {
				log.Printf("Heartbeat failed: %v", err)
			}
		}
	}
}

func (c *Client) sendHeartbeat() error {
	c.mu.RLock()
	stationID := c.stationID
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return fmt.Errorf("not connected")
	}

	req := &pb.HeartbeatRequest{
		StationId: stationID,
		Status:    "online",
		Metrics: &pb.SystemMetrics{
			CpuUsage:    0.0,
			MemoryUsage: 0.0,
			DiskUsage:   0.0,
			NetworkIn:   0,
			NetworkOut:  0,
		},
	}

	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()

	resp, err := c.client.Heartbeat(ctx, req)
	if err != nil {
		return err
	}

	if !resp.Success {
		log.Printf("Heartbeat warning: %s", resp.Message)
	}

	return nil
}

func (c *Client) Stop() {
	if !viper.GetBool("cloudshipai.enabled") {
		return
	}

	log.Printf("Stopping CloudShip AI client")
	c.cancel()
	c.disconnect()
	c.wg.Wait()
	log.Printf("CloudShip AI client stopped")
}

func (c *Client) disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false
	
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// detectDeploymentType determines the deployment environment and returns appropriate hardware info
func detectDeploymentType() *pb.HardwareInfo {
	// Check for Kubernetes
	if isKubernetes() {
		return detectKubernetesInfo()
	}
	
	// Check for ECS
	if isECS() {
		return detectECSInfo()
	}
	
	// Check for Docker
	if isDocker() {
		return detectDockerInfo()
	}
	
	// Check for Lambda/Serverless
	if isServerless() {
		return detectServerlessInfo()
	}
	
	// Default to bare metal/VM
	return detectBaremetalInfo()
}

func isKubernetes() bool {
	return os.Getenv("KUBERNETES_SERVICE_HOST") != ""
}

func isECS() bool {
	return os.Getenv("ECS_CONTAINER_METADATA_URI") != "" || 
		   os.Getenv("AWS_EXECUTION_ENV") == "AWS_ECS_FARGATE" ||
		   os.Getenv("AWS_EXECUTION_ENV") == "AWS_ECS_EC2"
}

func isDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

func isServerless() bool {
	return os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" ||
		   os.Getenv("LAMBDA_RUNTIME_DIR") != "" ||
		   os.Getenv("FUNCTIONS_WORKER_RUNTIME") != "" // Azure
}

func detectKubernetesInfo() *pb.HardwareInfo {
	hostname, _ := os.Hostname()
	
	additional := make(map[string]string)
	additional["deployment_type"] = "kubernetes"
	additional["k8s_pod_name"] = hostname
	
	if namespace := os.Getenv("POD_NAMESPACE"); namespace != "" {
		additional["k8s_namespace"] = namespace
	}
	if serviceName := os.Getenv("SERVICE_NAME"); serviceName != "" {
		additional["k8s_service_name"] = serviceName
	}
	if clusterName := os.Getenv("CLUSTER_NAME"); clusterName != "" {
		additional["k8s_cluster_name"] = clusterName
	}
	if nodeName := os.Getenv("NODE_NAME"); nodeName != "" {
		additional["k8s_node_name"] = nodeName
	}
	if pvcName := os.Getenv("PVC_NAME"); pvcName != "" {
		additional["persistent_volume_id"] = pvcName
	}
	
	return &pb.HardwareInfo{
		Cpu:        runtime.GOARCH,
		Memory:     "kubernetes",
		Disk:       hostname,
		Gpu:        "none",
		Additional: additional,
	}
}

func detectECSInfo() *pb.HardwareInfo {
	additional := make(map[string]string)
	additional["deployment_type"] = "ecs"
	
	if clusterArn := os.Getenv("ECS_CLUSTER"); clusterArn != "" {
		additional["ecs_cluster"] = clusterArn
	}
	if serviceName := os.Getenv("ECS_SERVICE_NAME"); serviceName != "" {
		additional["ecs_service_name"] = serviceName
	}
	if taskArn := os.Getenv("ECS_TASK_ARN"); taskArn != "" {
		additional["task_arn"] = taskArn
	}
	if taskDefArn := os.Getenv("ECS_TASK_DEFINITION_ARN"); taskDefArn != "" {
		additional["task_definition_arn"] = taskDefArn
	}
	if containerName := os.Getenv("ECS_CONTAINER_NAME"); containerName != "" {
		additional["ecs_container_name"] = containerName
	}
	if region := os.Getenv("AWS_REGION"); region != "" {
		additional["aws_region"] = region
	}
	if volumeId := os.Getenv("EBS_VOLUME_ID"); volumeId != "" {
		additional["ebs_volume_id"] = volumeId
	}
	
	return &pb.HardwareInfo{
		Cpu:        runtime.GOARCH,
		Memory:     "ecs",
		Disk:       "ecs-task",
		Gpu:        "none",
		Additional: additional,
	}
}

func detectDockerInfo() *pb.HardwareInfo {
	hostname, _ := os.Hostname()
	
	additional := make(map[string]string)
	additional["deployment_type"] = "docker"
	additional["container_id"] = hostname // Docker sets hostname to container ID
	
	if containerName := os.Getenv("CONTAINER_NAME"); containerName != "" {
		additional["container_name"] = containerName
	}
	if imageDigest := os.Getenv("IMAGE_DIGEST"); imageDigest != "" {
		additional["image_digest"] = imageDigest
	}
	if dockerHost := os.Getenv("DOCKER_HOST"); dockerHost != "" {
		additional["docker_host"] = dockerHost
	}
	if volumeId := os.Getenv("PERSISTENT_VOLUME_ID"); volumeId != "" {
		additional["persistent_volume_id"] = volumeId
	}
	
	return &pb.HardwareInfo{
		Cpu:        runtime.GOARCH,
		Memory:     "docker",
		Disk:       hostname,
		Gpu:        "none",
		Additional: additional,
	}
}

func detectServerlessInfo() *pb.HardwareInfo {
	additional := make(map[string]string)
	additional["deployment_type"] = "serverless"
	
	if functionName := os.Getenv("AWS_LAMBDA_FUNCTION_NAME"); functionName != "" {
		additional["function_name"] = functionName
		additional["runtime_environment"] = "aws-lambda"
		if version := os.Getenv("AWS_LAMBDA_FUNCTION_VERSION"); version != "" {
			additional["function_version"] = version
		}
		if region := os.Getenv("AWS_REGION"); region != "" {
			additional["aws_region"] = region
		}
	} else if functionName := os.Getenv("FUNCTIONS_WORKER_RUNTIME"); functionName != "" {
		additional["runtime_environment"] = "azure-functions"
	}
	
	// Generate execution environment ID for cold starts
	additional["execution_environment_id"] = fmt.Sprintf("%d", time.Now().UnixNano())
	
	return &pb.HardwareInfo{
		Cpu:        runtime.GOARCH,
		Memory:     "serverless",
		Disk:       "ephemeral",
		Gpu:        "none",
		Additional: additional,
	}
}

func detectBaremetalInfo() *pb.HardwareInfo {
	hostname, _ := os.Hostname()
	
	additional := make(map[string]string)
	additional["deployment_type"] = "bare_metal"
	additional["hostname"] = hostname
	
	// Try to read machine ID (Linux)
	if machineId, err := ioutil.ReadFile("/etc/machine-id"); err == nil {
		additional["machine_id"] = strings.TrimSpace(string(machineId))
	}
	
	// Try to get disk serial (simplified)
	additional["disk_serial"] = "unknown"
	
	// IP address will be determined in register function
	
	return &pb.HardwareInfo{
		Cpu:        getCPUInfo(),
		Memory:     getMemoryInfo(),
		Disk:       hostname,
		Gpu:        "none",
		Additional: additional,
	}
}

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func getCPUInfo() string {
	// Simplified CPU detection
	return fmt.Sprintf("%s (%d cores)", runtime.GOARCH, runtime.NumCPU())
}

func getMemoryInfo() string {
	// This is simplified - in production you'd want actual memory info
	return "unknown"
}