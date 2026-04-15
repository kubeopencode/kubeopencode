// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

const (
	// DefaultSkillsMountBase is the base directory where skills are mounted in agent pods.
	// Each SkillSource gets its own subdirectory: /skills/{source-name}/
	DefaultSkillsMountBase = "/skills"

	// DefaultPluginsMountBase is the base directory where plugins are installed.
	// The plugin-init container runs npm install here, creating /plugins/node_modules/.
	DefaultPluginsMountBase = "/plugins"
)

// processSkills converts SkillSource items into gitMounts and returns
// the list of directory paths where skills will be available.
// These paths are later injected into OpenCode's skills.paths configuration.
func processSkills(skills []kubeopenv1alpha1.SkillSource) ([]gitMount, []string) {
	if len(skills) == 0 {
		return nil, nil
	}

	var gitMounts []gitMount
	var skillPaths []string

	for _, s := range skills {
		if s.Git == nil {
			continue
		}
		git := s.Git

		mountPath := filepath.Join(DefaultSkillsMountBase, s.Name)

		depth := DefaultGitDepth
		if git.Depth != nil && *git.Depth >= 0 {
			depth = *git.Depth
		}

		ref := defaultString(git.Ref, DefaultGitRef)

		secretName := ""
		if git.SecretRef != nil {
			secretName = git.SecretRef.Name
		}

		gm := gitMount{
			contextName:       "skill-" + s.Name,
			repository:        git.Repository,
			ref:               ref,
			repoPath:          git.Path,
			mountPath:         mountPath,
			depth:             depth,
			secretName:        secretName,
			recurseSubmodules: git.RecurseSubmodules,
			names:             git.Names,
		}
		gitMounts = append(gitMounts, gm)

		// Compute skill paths for OpenCode discovery.
		// If Names is empty, OpenCode scans the entire mount point for **/SKILL.md.
		// If Names is specified, we point to each specific skill directory.
		if len(git.Names) == 0 {
			skillPaths = append(skillPaths, mountPath)
		} else {
			for _, name := range git.Names {
				skillPaths = append(skillPaths, filepath.Join(mountPath, name))
			}
		}
	}

	return gitMounts, skillPaths
}

// processSkillsPluginsAndInjectConfig handles the full skill and plugin processing pipeline:
// converts SkillSources to git mounts, injects skills.paths and plugins into the OpenCode config,
// and adds the config to the ConfigMap data.
//
// Note: This function does NOT validate JSON syntax. The caller (Task controller)
// is responsible for JSON validation when needed. The Agent controller intentionally
// skips validation to allow Deployment creation even with invalid config (the error
// surfaces at Task execution time instead).
func processSkillsPluginsAndInjectConfig(skills []kubeopenv1alpha1.SkillSource, plugins []kubeopenv1alpha1.PluginSpec, config *runtime.RawExtension, configMapData map[string]string, fileMounts []fileMount) ([]gitMount, []fileMount, error) {
	skillGitMounts, skillPaths := processSkills(skills)

	// Rewrite plugin names to file:// paths pointing to the shared /plugins volume
	// where plugin-init container has installed them via npm
	resolvedPlugins := resolvePluginPaths(plugins)

	// Split resolved plugins by target
	serverPlugins, tuiPlugins := splitPluginsByTarget(resolvedPlugins)

	effectiveConfig := config
	if len(skillPaths) > 0 {
		injected, err := injectSkillsIntoConfig(effectiveConfig, skillPaths)
		if err != nil {
			return nil, fileMounts, fmt.Errorf("failed to inject skills config: %w", err)
		}
		effectiveConfig = injected
	}

	// Inject server plugins into opencode.json
	if len(serverPlugins) > 0 {
		injected, err := injectPluginsIntoConfig(effectiveConfig, serverPlugins)
		if err != nil {
			return nil, fileMounts, fmt.Errorf("failed to inject server plugins config: %w", err)
		}
		effectiveConfig = injected
	}

	if !configIsEmpty(effectiveConfig) {
		configMapKey := sanitizeConfigMapKey(OpenCodeConfigPath)
		configMapData[configMapKey] = string(effectiveConfig.Raw)
		fileMounts = append(fileMounts, fileMount{filePath: OpenCodeConfigPath})
	}

	// Inject TUI plugins into tui.json (separate config file)
	if len(tuiPlugins) > 0 {
		tuiConfig, err := injectPluginsIntoConfig(nil, tuiPlugins)
		if err != nil {
			return nil, fileMounts, fmt.Errorf("failed to inject TUI plugins config: %w", err)
		}
		if !configIsEmpty(tuiConfig) {
			configMapKey := sanitizeConfigMapKey(OpenCodeTUIConfigPath)
			configMapData[configMapKey] = string(tuiConfig.Raw)
			fileMounts = append(fileMounts, fileMount{filePath: OpenCodeTUIConfigPath})
		}
	}

	return skillGitMounts, fileMounts, nil
}

// resolvePluginPaths rewrites plugin npm package names to file:// paths
// pointing to the shared /plugins volume where the plugin-init container
// installs them. The file path format is:
//
//	file:///plugins/node_modules/<package-name>
//
// For scoped packages (e.g. @org/plugin), npm installs them at:
//
//	/plugins/node_modules/@org/plugin
//
// so the file:// path preserves the scope.
func resolvePluginPaths(plugins []kubeopenv1alpha1.PluginSpec) []kubeopenv1alpha1.PluginSpec {
	if len(plugins) == 0 {
		return nil
	}

	seen := make(map[string]bool, len(plugins))
	resolved := make([]kubeopenv1alpha1.PluginSpec, 0, len(plugins))

	for _, p := range plugins {
		// Extract bare package name (strip version specifier for dedup and path)
		pkgName := extractNpmPackageName(p.Name)
		if seen[pkgName] {
			continue // skip duplicates (first wins)
		}
		seen[pkgName] = true

		rewritten := kubeopenv1alpha1.PluginSpec{
			Name:    "file://" + filepath.Join(DefaultPluginsMountBase, "node_modules", pkgName),
			Target:  p.Target,
			Options: p.Options,
		}
		resolved = append(resolved, rewritten)
	}

	return resolved
}

// extractNpmPackageName extracts the bare package name from an npm specifier,
// stripping any version suffix. Examples:
//
//	"cc-safety-net"         → "cc-safety-net"
//	"cc-safety-net@0.8.2"  → "cc-safety-net"
//	"@org/plugin@^1.0.0"   → "@org/plugin"
//	"@org/plugin"           → "@org/plugin"
func extractNpmPackageName(spec string) string {
	// Scoped package: @scope/name or @scope/name@version
	if strings.HasPrefix(spec, "@") {
		// Find the second @ (version separator) after the scope
		rest := spec[1:] // skip leading @
		if idx := strings.Index(rest, "@"); idx != -1 {
			return spec[:idx+1] // +1 for the leading @
		}
		return spec
	}
	// Unscoped: name or name@version
	if idx := strings.Index(spec, "@"); idx != -1 {
		return spec[:idx]
	}
	return spec
}

// splitPluginsByTarget separates plugins into server and TUI groups.
// Plugins with empty or "server" target go to server; "tui" target goes to TUI.
func splitPluginsByTarget(plugins []kubeopenv1alpha1.PluginSpec) (server, tui []kubeopenv1alpha1.PluginSpec) {
	for _, p := range plugins {
		if p.Target == kubeopenv1alpha1.PluginTargetTUI {
			tui = append(tui, p)
		} else {
			server = append(server, p)
		}
	}
	return
}

// hasServerPlugins returns true if any plugin targets the server runtime.
func hasServerPlugins(plugins []kubeopenv1alpha1.PluginSpec) bool {
	for _, p := range plugins {
		if p.Target != kubeopenv1alpha1.PluginTargetTUI {
			return true
		}
	}
	return false
}

// hasTUIPlugins returns true if any plugin targets the TUI runtime.
func hasTUIPlugins(plugins []kubeopenv1alpha1.PluginSpec) bool {
	for _, p := range plugins {
		if p.Target == kubeopenv1alpha1.PluginTargetTUI {
			return true
		}
	}
	return false
}

// injectPluginsIntoConfig merges plugin entries from spec.plugins into an existing
// OpenCode configuration object. If existingConfig is nil or empty, a new config
// object is created.
//
// OpenCode plugin array format:
//   - Plugin without options: string entry, e.g. "@org/plugin-name"
//   - Plugin with options: tuple entry, e.g. ["@org/plugin-name", { "key": "value" }]
//
// Plugins from spec.plugins are appended to any existing plugins in the config.
// Duplicate detection is by package name (first occurrence wins).
func injectPluginsIntoConfig(existingConfig *runtime.RawExtension, plugins []kubeopenv1alpha1.PluginSpec) (*runtime.RawExtension, error) {
	if len(plugins) == 0 {
		return existingConfig, nil
	}

	// Parse existing config or start fresh
	configMap := make(map[string]interface{})
	if !configIsEmpty(existingConfig) {
		if err := json.Unmarshal(existingConfig.Raw, &configMap); err != nil {
			return nil, fmt.Errorf("failed to parse existing config: %w", err)
		}
	}

	// Collect existing plugin names for deduplication
	existingPluginNames := make(map[string]bool)
	var allPlugins []interface{}
	if pluginsRaw, ok := configMap["plugin"]; ok {
		if pluginsArr, ok := pluginsRaw.([]interface{}); ok {
			for _, p := range pluginsArr {
				allPlugins = append(allPlugins, p)
				// Extract name from string or tuple format
				switch v := p.(type) {
				case string:
					existingPluginNames[v] = true
				case []interface{}:
					if len(v) > 0 {
						if name, ok := v[0].(string); ok {
							existingPluginNames[name] = true
						}
					}
				}
			}
		}
	}

	// Append new plugins (deduplicate by name)
	added := false
	for _, p := range plugins {
		if existingPluginNames[p.Name] {
			continue
		}
		if p.Options != nil && p.Options.Raw != nil {
			// Plugin with options: ["name", { options }]
			var opts interface{}
			if err := json.Unmarshal(p.Options.Raw, &opts); err != nil {
				return nil, fmt.Errorf("failed to parse plugin options for %q: %w", p.Name, err)
			}
			allPlugins = append(allPlugins, []interface{}{p.Name, opts})
		} else {
			// Plugin without options: "name"
			allPlugins = append(allPlugins, p.Name)
		}
		existingPluginNames[p.Name] = true
		added = true
	}

	// If no new plugins were added, return the original config unchanged
	if !added && existingConfig != nil {
		return existingConfig, nil
	}

	configMap["plugin"] = allPlugins

	result, err := json.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config with plugins: %w", err)
	}

	return &runtime.RawExtension{Raw: result}, nil
}

// injectSkillsIntoConfig merges skills.paths entries into an existing OpenCode
// configuration object. If existingConfig is nil or empty, a new config
// object is created. Existing skills.paths entries are preserved (appended, deduplicated).
// Other fields in the config (including skills.urls) are preserved.
func injectSkillsIntoConfig(existingConfig *runtime.RawExtension, skillPaths []string) (*runtime.RawExtension, error) {
	if len(skillPaths) == 0 {
		return existingConfig, nil
	}

	// Parse existing config or start fresh
	configMap := make(map[string]interface{})
	if !configIsEmpty(existingConfig) {
		if err := json.Unmarshal(existingConfig.Raw, &configMap); err != nil {
			return nil, fmt.Errorf("failed to parse existing config: %w", err)
		}
	}

	// Get or create "skills" object
	skillsObj, _ := configMap["skills"].(map[string]interface{})
	if skillsObj == nil {
		skillsObj = make(map[string]interface{})
	}

	// Collect existing paths in original order (preserving order prevents
	// non-deterministic JSON output which would cause infinite reconciliation loops)
	existingPaths := make(map[string]bool)
	var allPaths []string
	if pathsRaw, ok := skillsObj["paths"]; ok {
		if pathsArr, ok := pathsRaw.([]interface{}); ok {
			for _, p := range pathsArr {
				if ps, ok := p.(string); ok {
					allPaths = append(allPaths, ps)
					existingPaths[ps] = true
				}
			}
		}
	}

	// Append new paths (deduplicate)
	added := false
	for _, p := range skillPaths {
		if !existingPaths[p] {
			allPaths = append(allPaths, p)
			added = true
		}
	}

	// If no new paths were added, return the original config unchanged
	// to avoid unnecessary JSON marshal and ConfigMap updates
	if !added && existingConfig != nil {
		return existingConfig, nil
	}

	skillsObj["paths"] = allPaths
	configMap["skills"] = skillsObj

	result, err := json.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config with skills: %w", err)
	}

	return &runtime.RawExtension{Raw: result}, nil
}
