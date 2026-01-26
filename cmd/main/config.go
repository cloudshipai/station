package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"station/internal/config"
	"station/internal/services"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	configCmd = &cobra.Command{
		Use:   "config",
		Short: "Manage Station configuration",
		Long: `View and modify Station configuration settings.

The config command respects the workspace location:
  - If --config flag is set, uses that file
  - If STATION_CONFIG_DIR is set, uses that directory
  - If 'workspace' is configured, uses that path
  - Otherwise uses the default XDG config directory

Examples:
  stn config show                           # Show all config
  stn config show coding                    # Show coding section
  stn config set ai_provider anthropic      # Set a value
  stn config set coding.backend claudecode  # Set nested value
  stn config reset coding.claudecode        # Reset to defaults
  stn config --browser                      # Edit in browser`,
		RunE: runConfigRoot,
	}

	configShowCmd = &cobra.Command{
		Use:   "show [section]",
		Short: "Show current configuration",
		Long: `Display current configuration values.

Optionally filter by section: ai, coding, cloudship, telemetry, sandbox, webhook, notifications, server`,
		Args: cobra.MaximumNArgs(1),
		RunE: runConfigShow,
	}

	configSetCmd = &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration value and save to config.yaml.

Keys use dot notation for nested values:
  ai_provider              - top level key
  coding.backend           - nested key
  coding.claudecode.model  - deeply nested key

Examples:
  stn config set ai_provider anthropic
  stn config set ai_model claude-sonnet-4-20250514
  stn config set coding.backend claudecode
  stn config set coding.claudecode.max_turns 20
  stn config set cloudship.enabled true
  stn config set cloudship.tags "prod,us-east-1"`,
		Args: cobra.ExactArgs(2),
		RunE: runConfigSet,
	}

	configResetCmd = &cobra.Command{
		Use:   "reset <key>",
		Short: "Reset a configuration value to default",
		Long: `Reset a configuration key to its default value.

Can reset individual keys or entire sections:
  stn config reset coding.claudecode.max_turns  # Reset single key
  stn config reset coding                       # Reset entire section`,
		Args: cobra.ExactArgs(1),
		RunE: runConfigReset,
	}

	configSchemaCmd = &cobra.Command{
		Use:   "schema",
		Short: "Show configuration schema",
		Long:  `Display all available configuration keys with their types, descriptions, and defaults.`,
		RunE:  runConfigSchema,
	}

	configPathCmd = &cobra.Command{
		Use:   "path",
		Short: "Show config file path",
		Long:  `Display the path to the config file that would be used.`,
		RunE:  runConfigPath,
	}

	configEditCmd = &cobra.Command{
		Use:   "edit",
		Short: "Edit configuration file in editor",
		RunE:  runConfigEdit,
	}
)

func initConfigCmd() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configResetCmd)
	configCmd.AddCommand(configSchemaCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configEditCmd)

	configCmd.Flags().Bool("browser", false, "Open browser for configuration editing")
	configShowCmd.Flags().Bool("json", false, "Output in JSON format")
	configSchemaCmd.Flags().Bool("json", false, "Output in JSON format")
}

func runConfigRoot(cmd *cobra.Command, args []string) error {
	browserMode, _ := cmd.Flags().GetBool("browser")
	if !browserMode {
		return cmd.Help()
	}

	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		configPath = filepath.Join(getWorkspacePath(), "config.yaml")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	browserConfig := services.NewBrowserConfigService(cfg.APIPort)
	return browserConfig.EditWithBrowser(cmd.Context(), configPath)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")

	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		configPath = filepath.Join(getWorkspacePath(), "config.yaml")
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("No config file found at: %s\n", configPath)
			fmt.Println("Run 'stn init' to create one, or 'stn config set <key> <value>' to create with defaults.")
			return nil
		}
		return fmt.Errorf("failed to read config: %w", err)
	}

	var configMap map[string]interface{}
	if err := yaml.Unmarshal(content, &configMap); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if len(args) > 0 {
		section := args[0]
		if sectionData, ok := configMap[section]; ok {
			configMap = map[string]interface{}{section: sectionData}
		} else {
			topLevelKeys := filterTopLevelKeys(configMap, section)
			if len(topLevelKeys) > 0 {
				configMap = topLevelKeys
			} else {
				return fmt.Errorf("section '%s' not found", section)
			}
		}
	}

	configMap = redactSecrets(configMap)

	if jsonOutput {
		output, _ := json.MarshalIndent(configMap, "", "  ")
		fmt.Println(string(output))
	} else {
		fmt.Printf("Config file: %s\n\n", configPath)
		output, _ := yaml.Marshal(configMap)
		fmt.Print(string(output))
	}

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	field := config.GetFieldByKey(key)
	if field == nil {
		fmt.Printf("Warning: '%s' is not a known config key. Setting anyway.\n", key)
	}

	var typedValue interface{}
	if field != nil {
		var err error
		typedValue, err = parseValueForType(value, field.Type)
		if err != nil {
			return fmt.Errorf("invalid value for %s (%s): %w", key, field.Type, err)
		}
	} else {
		typedValue = inferValue(value)
	}

	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		configPath = filepath.Join(getWorkspacePath(), "config.yaml")
	}

	var configMap map[string]interface{}
	content, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			configMap = make(map[string]interface{})
		} else {
			return fmt.Errorf("failed to read config: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(content, &configMap); err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}
	}

	setNestedValue(configMap, key, typedValue)

	output, err := yaml.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Printf("✅ Set %s = %v\n", key, typedValue)
	fmt.Printf("   Config file: %s\n", configPath)

	return nil
}

func runConfigReset(cmd *cobra.Command, args []string) error {
	key := args[0]

	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		configPath = filepath.Join(getWorkspacePath(), "config.yaml")
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var configMap map[string]interface{}
	if err := yaml.Unmarshal(content, &configMap); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if strings.Contains(key, ".") {
		parts := strings.Split(key, ".")
		deleteNestedKey(configMap, parts)
	} else {
		delete(configMap, key)
	}

	fields := config.GetFieldsBySection(key)
	if len(fields) > 0 {
		delete(configMap, key)
		fmt.Printf("✅ Reset section '%s' to defaults\n", key)
	} else {
		field := config.GetFieldByKey(key)
		if field != nil && field.Default != nil {
			setNestedValue(configMap, key, field.Default)
			fmt.Printf("✅ Reset %s to default: %v\n", key, field.Default)
		} else {
			fmt.Printf("✅ Removed %s\n", key)
		}
	}

	output, err := yaml.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := os.WriteFile(configPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Printf("   Config file: %s\n", configPath)
	return nil
}

func runConfigSchema(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")

	sections := config.GetConfigSections()
	schema := config.GetConfigSchema()

	if jsonOutput {
		output := map[string]interface{}{
			"sections": sections,
			"fields":   schema,
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	sort.Slice(sections, func(i, j int) bool {
		return sections[i].Order < sections[j].Order
	})

	for _, section := range sections {
		fields := config.GetFieldsBySection(section.Name)
		if len(fields) == 0 {
			continue
		}

		fmt.Printf("\n%s (%s)\n", section.Description, section.Name)
		fmt.Println(strings.Repeat("-", 60))

		for _, f := range fields {
			defaultStr := ""
			if f.Default != nil {
				defaultStr = fmt.Sprintf(" (default: %v)", f.Default)
			}
			secretStr := ""
			if f.Secret {
				secretStr = " [secret]"
			}
			optionsStr := ""
			if len(f.Options) > 0 {
				optionsStr = fmt.Sprintf(" [%s]", strings.Join(f.Options, "|"))
			}

			fmt.Printf("  %-40s %s%s%s%s\n", f.Key, f.Type, defaultStr, secretStr, optionsStr)
			fmt.Printf("    %s\n", f.Description)
		}
	}

	return nil
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		configPath = filepath.Join(getWorkspacePath(), "config.yaml")
	}

	fmt.Printf("Config file: %s\n", configPath)

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("Status: does not exist (will be created on first 'stn config set')")
	} else {
		fmt.Println("Status: exists")
	}

	return nil
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = filepath.Join(getWorkspacePath(), "config.yaml")
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano"
	}

	fmt.Printf("Opening config file with %s: %s\n", editor, configFile)

	editorCmd := exec.Command(editor, configFile)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	return editorCmd.Run()
}

func parseValueForType(value string, fieldType config.FieldType) (interface{}, error) {
	switch fieldType {
	case config.FieldTypeString:
		return value, nil
	case config.FieldTypeInt:
		return strconv.Atoi(value)
	case config.FieldTypeBool:
		return strconv.ParseBool(value)
	case config.FieldTypeStringSlice:
		if value == "" {
			return []string{}, nil
		}
		return strings.Split(value, ","), nil
	default:
		return value, nil
	}
}

func inferValue(value string) interface{} {
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	if b, err := strconv.ParseBool(value); err == nil {
		return b
	}
	if strings.Contains(value, ",") {
		return strings.Split(value, ",")
	}
	return value
}

func setNestedValue(m map[string]interface{}, key string, value interface{}) {
	parts := strings.Split(key, ".")
	current := m

	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}

		if _, ok := current[part]; !ok {
			current[part] = make(map[string]interface{})
		}

		if nested, ok := current[part].(map[string]interface{}); ok {
			current = nested
		} else {
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		}
	}
}

func deleteNestedKey(m map[string]interface{}, parts []string) {
	if len(parts) == 1 {
		delete(m, parts[0])
		return
	}

	if nested, ok := m[parts[0]].(map[string]interface{}); ok {
		deleteNestedKey(nested, parts[1:])
		if len(nested) == 0 {
			delete(m, parts[0])
		}
	}
}

func filterTopLevelKeys(m map[string]interface{}, prefix string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		if strings.HasPrefix(k, prefix) {
			result[k] = v
		}
	}
	return result
}

func redactSecrets(m map[string]interface{}) map[string]interface{} {
	secretKeys := map[string]bool{
		"ai_api_key":             true,
		"api_key":                true,
		"registration_key":       true,
		"token":                  true,
		"ai_oauth_token":         true,
		"ai_oauth_refresh_token": true,
		"password":               true,
		"identity_token":         true,
	}

	return redactSecretsRecursive(m, secretKeys)
}

func redactSecretsRecursive(m map[string]interface{}, secretKeys map[string]bool) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		if secretKeys[k] {
			if str, ok := v.(string); ok && str != "" {
				result[k] = "***REDACTED***"
			} else {
				result[k] = v
			}
		} else if nested, ok := v.(map[string]interface{}); ok {
			result[k] = redactSecretsRecursive(nested, secretKeys)
		} else {
			result[k] = v
		}
	}
	return result
}
