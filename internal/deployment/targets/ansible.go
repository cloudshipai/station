package targets

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"station/internal/deployment"
)

type AnsibleTarget struct{}

func NewAnsibleTarget() *AnsibleTarget {
	return &AnsibleTarget{}
}

func (a *AnsibleTarget) Name() string {
	return "ansible"
}

func (a *AnsibleTarget) Validate(ctx context.Context) error {
	if _, err := exec.LookPath("ansible-playbook"); err != nil {
		return fmt.Errorf("ansible-playbook not found: install with 'pip install ansible'")
	}
	return nil
}

func (a *AnsibleTarget) GenerateConfig(ctx context.Context, config *deployment.DeploymentConfig, secrets map[string]string) (map[string]string, error) {
	return a.GenerateConfigWithOptions(ctx, config, secrets, deployment.DeployOptions{})
}

func (a *AnsibleTarget) GenerateConfigWithOptions(ctx context.Context, config *deployment.DeploymentConfig, secrets map[string]string, options deployment.DeployOptions) (map[string]string, error) {
	files := make(map[string]string)
	appName := fmt.Sprintf("station-%s", config.EnvironmentName)

	files["inventory.ini"] = a.generateInventory(options)
	files["playbook.yml"] = a.generatePlaybook(appName, config, options)
	files["vars/main.yml"] = a.generateVars(appName, config, secrets, options)
	files["templates/docker-compose.yml.j2"] = a.generateDockerComposeTemplate(appName, config, options)
	files["templates/station.service.j2"] = a.generateSystemdTemplate(appName)

	return files, nil
}

func (a *AnsibleTarget) Deploy(ctx context.Context, config *deployment.DeploymentConfig, secrets map[string]string, options deployment.DeployOptions) error {
	files, err := a.GenerateConfigWithOptions(ctx, config, secrets, options)
	if err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	outputDir := options.OutputDir
	if outputDir == "" {
		outputDir = fmt.Sprintf("ansible-%s", config.EnvironmentName)
	}

	for _, subdir := range []string{"vars", "templates"} {
		if err := os.MkdirAll(fmt.Sprintf("%s/%s", outputDir, subdir), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
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
		fmt.Printf("   To run: ansible-playbook -i %s/inventory.ini %s/playbook.yml\n", outputDir, outputDir)
		return nil
	}

	fmt.Printf("\n🚀 Running Ansible playbook...\n")

	args := []string{"-i", fmt.Sprintf("%s/inventory.ini", outputDir), fmt.Sprintf("%s/playbook.yml", outputDir)}
	if options.AutoApprove {
		args = append(args, "--diff")
	} else {
		args = append(args, "--diff", "--check")
		fmt.Printf("Running in check mode first (failures in check mode for initial deployments are expected)...\n")
	}

	checkCmd := exec.CommandContext(ctx, "ansible-playbook", args...)
	checkCmd.Stdout = os.Stdout
	checkCmd.Stderr = os.Stderr

	checkErr := checkCmd.Run()
	if checkErr != nil && !options.AutoApprove {
		fmt.Printf("\n⚠️  Check mode completed with errors (this is normal for initial deployments)\n")
	}

	if !options.AutoApprove {
		fmt.Printf("\nProceed with deployment? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			return fmt.Errorf("deployment cancelled by user")
		}

		runArgs := []string{"-i", fmt.Sprintf("%s/inventory.ini", outputDir), fmt.Sprintf("%s/playbook.yml", outputDir), "--diff"}
		runCmd := exec.CommandContext(ctx, "ansible-playbook", runArgs...)
		runCmd.Stdout = os.Stdout
		runCmd.Stderr = os.Stderr

		if err := runCmd.Run(); err != nil {
			return fmt.Errorf("ansible playbook failed: %w", err)
		}
	}

	fmt.Printf("\n✅ Deployment complete!\n")
	return nil
}

func (a *AnsibleTarget) Destroy(ctx context.Context, config *deployment.DeploymentConfig) error {
	outputDir := fmt.Sprintf("ansible-%s", config.EnvironmentName)

	fmt.Printf("🗑️  Running Ansible teardown...\n")

	args := []string{
		"-i", fmt.Sprintf("%s/inventory.ini", outputDir),
		fmt.Sprintf("%s/playbook.yml", outputDir),
		"-e", "station_state=absent",
		"--diff",
	}

	cmd := exec.CommandContext(ctx, "ansible-playbook", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ansible teardown failed: %w", err)
	}

	fmt.Printf("✅ Teardown complete\n")
	return nil
}

func (a *AnsibleTarget) Status(ctx context.Context, config *deployment.DeploymentConfig) (*deployment.DeploymentStatus, error) {
	return &deployment.DeploymentStatus{
		State:   "unknown",
		Message: "Ansible deployments require SSH access to check status. Use 'ansible -m shell -a \"docker ps\" <host>' to check.",
	}, nil
}

func (a *AnsibleTarget) generateInventory(options deployment.DeployOptions) string {
	var sb strings.Builder
	sb.WriteString("[station_servers]\n")

	if len(options.Hosts) > 0 {
		for _, host := range options.Hosts {
			var hostAddr, user string

			if strings.Contains(host, "@") {
				parts := strings.SplitN(host, "@", 2)
				user = parts[0]
				hostAddr = parts[1]
			} else {
				hostAddr = host
				user = options.SSHUser
				if user == "" {
					user = "root"
				}
			}

			hostLine := fmt.Sprintf("%s ansible_user=%s", hostAddr, user)
			if options.SSHKey != "" {
				hostLine = fmt.Sprintf("%s ansible_ssh_private_key_file=%s", hostLine, options.SSHKey)
			}
			sb.WriteString(hostLine + "\n")
		}
	} else {
		sb.WriteString("# Add your target hosts here, or use --hosts flag\n")
		sb.WriteString("# Example:\n")
		sb.WriteString("#   stn deploy myenv --target ansible --hosts user@server1,user@server2\n")
		sb.WriteString("#   stn deploy myenv --target ansible --hosts server1 --ssh-user ubuntu --ssh-key ~/.ssh/id_rsa\n")
		sb.WriteString("# \n")
		sb.WriteString("# For local deployment (using Docker):\n")
		sb.WriteString("# localhost ansible_connection=local\n")
	}

	sb.WriteString("\n[station_servers:vars]\n")
	sb.WriteString("ansible_python_interpreter=/usr/bin/python3\n")
	sb.WriteString("ansible_ssh_common_args='-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'\n")

	return sb.String()
}

func (a *AnsibleTarget) generatePlaybook(appName string, config *deployment.DeploymentConfig, options deployment.DeployOptions) string {
	bundleCopyTask := ""
	if options.BundlePath != "" {
		bundleCopyTask = `
    - name: Create Station bundle directory
      file:
        path: "{{ station_bundle_dir }}"
        state: directory
        mode: '0755'
        owner: '1000'
        group: '1000'

    - name: Copy bundle file to remote host
      copy:
        src: "{{ station_bundle_src }}"
        dest: "{{ station_bundle_dir }}/bundle.tar.gz"
        mode: '0644'
        owner: '1000'
        group: '1000'
`
	}

	return fmt.Sprintf(`---
- name: Deploy Station %s
  hosts: station_servers
  become: yes
  vars_files:
    - vars/main.yml

  tasks:
    - name: Ensure Docker is installed
      apt:
        name:
          - docker.io
          - docker-compose
        state: present
        update_cache: yes
      when: ansible_os_family == "Debian"

    - name: Ensure Docker is installed (RHEL)
      yum:
        name:
          - docker
          - docker-compose
        state: present
      when: ansible_os_family == "RedHat"

    - name: Start Docker service
      systemd:
        name: docker
        state: started
        enabled: yes

    - name: Create Station directory
      file:
        path: "{{ station_install_dir }}"
        state: directory
        mode: '0755'

    - name: Create Station data directory
      file:
        path: "{{ station_data_dir }}"
        state: directory
        mode: '0755'
        owner: '1000'
        group: '1000'
%s
    - name: Copy docker-compose file
      template:
        src: templates/docker-compose.yml.j2
        dest: "{{ station_install_dir }}/docker-compose.yml"
        mode: '0644'
      notify: Restart Station

    - name: Copy systemd service file
      template:
        src: templates/station.service.j2
        dest: /etc/systemd/system/station-{{ station_name }}.service
        mode: '0644'
      notify:
        - Reload systemd
        - Restart Station

    - name: Pull Station image
      docker_image:
        name: "{{ station_image }}"
        source: pull

    - name: Start Station service
      systemd:
        name: station-{{ station_name }}
        state: "{{ 'stopped' if station_state == 'absent' else 'started' }}"
        enabled: "{{ station_state != 'absent' }}"

    - name: Remove Station (when station_state=absent)
      block:
        - name: Stop containers
          community.docker.docker_compose_v2:
            project_src: "{{ station_install_dir }}"
            state: absent
          ignore_errors: yes

        - name: Remove install directory
          file:
            path: "{{ station_install_dir }}"
            state: absent

        - name: Remove systemd service
          file:
            path: /etc/systemd/system/station-{{ station_name }}.service
            state: absent
          notify: Reload systemd
      when: station_state == "absent"

  handlers:
    - name: Reload systemd
      systemd:
        daemon_reload: yes

    - name: Restart Station
      systemd:
        name: station-{{ station_name }}
        state: restarted
      when: station_state != "absent"
`, appName, bundleCopyTask)
}

func (a *AnsibleTarget) generateVars(appName string, config *deployment.DeploymentConfig, secrets map[string]string, options deployment.DeployOptions) string {
	var secretVars strings.Builder
	for key, value := range secrets {
		secretVars.WriteString(fmt.Sprintf("  %s: %q\n", key, value))
	}

	bundleVars := ""
	if options.BundlePath != "" {
		bundleVars = fmt.Sprintf(`station_bundle_dir: /opt/station/%s/bundle
station_bundle_src: %s
`, appName, options.BundlePath)
	}

	return fmt.Sprintf(`---
station_name: %s
station_image: %s
station_install_dir: /opt/station/%s
station_data_dir: /opt/station/%s/data
station_state: present
%s
station_mcp_port: 8586
station_dynamic_mcp_port: 8587

station_env:
%s
`, appName, config.DockerImage, appName, appName, bundleVars, secretVars.String())
}

func (a *AnsibleTarget) generateDockerComposeTemplate(appName string, config *deployment.DeploymentConfig, options deployment.DeployOptions) string {
	bundleVolume := ""
	bundleCommand := ""
	if options.BundlePath != "" {
		bundleVolume = `
      - {{ station_bundle_dir }}:/bundles:ro`
		bundleCommand = `
    command: >
      sh -c "
        if [ -f /bundles/bundle.tar.gz ]; then
          echo 'Installing bundle from /bundles/bundle.tar.gz...' &&
          stn bundle install /bundles/bundle.tar.gz default --force 2>/dev/null || true
        fi &&
        exec stn serve
      "`
	}

	return fmt.Sprintf(`services:
  station:
    image: {{ station_image }}
    container_name: %s
    restart: unless-stopped
    ports:
      - "{{ station_mcp_port }}:8586"
      - "{{ station_dynamic_mcp_port }}:8587"
    environment:
{%% for key, value in station_env.items() %%}
      - {{ key }}={{ value }}
{%% endfor %%}
    volumes:
      - {{ station_data_dir }}:/home/station/.config/station%s%s
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8587/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
`, appName, bundleVolume, bundleCommand)
}

func (a *AnsibleTarget) generateSystemdTemplate(appName string) string {
	return fmt.Sprintf(`[Unit]
Description=Station %s
Requires=docker.service
After=docker.service

[Service]
Type=simple
WorkingDirectory={{ station_install_dir }}
ExecStart=/usr/bin/docker-compose up
ExecStop=/usr/bin/docker-compose down
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
`, appName)
}

func init() {
	deployment.RegisterDeploymentTarget(NewAnsibleTarget())
}
