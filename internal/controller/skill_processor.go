// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

const (
	// DefaultSkillsMountBase is the base directory where skills are mounted in agent pods.
	// Each SkillSource gets its own subdirectory: /skills/{source-name}/
	DefaultSkillsMountBase = "/skills"
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

// processSkillsAndInjectConfig handles the full skill processing pipeline:
// converts SkillSources to git mounts, injects skills.paths into the OpenCode config,
// and adds the config to the ConfigMap data.
//
// Note: This function does NOT validate JSON syntax. The caller (Task controller)
// is responsible for JSON validation when needed. The Agent controller intentionally
// skips validation to allow Deployment creation even with invalid config (the error
// surfaces at Task execution time instead).
func processSkillsAndInjectConfig(skills []kubeopenv1alpha1.SkillSource, config *string, configMapData map[string]string, fileMounts []fileMount) ([]gitMount, []fileMount, error) {
	skillGitMounts, skillPaths := processSkills(skills)

	effectiveConfig := config
	if len(skillPaths) > 0 {
		injected, err := injectSkillsIntoConfig(effectiveConfig, skillPaths)
		if err != nil {
			return nil, fileMounts, fmt.Errorf("failed to inject skills config: %w", err)
		}
		effectiveConfig = injected
	}

	if effectiveConfig != nil && *effectiveConfig != "" {
		configMapKey := sanitizeConfigMapKey(OpenCodeConfigPath)
		configMapData[configMapKey] = *effectiveConfig
		fileMounts = append(fileMounts, fileMount{filePath: OpenCodeConfigPath})
	}

	return skillGitMounts, fileMounts, nil
}

// injectSkillsIntoConfig merges skills.paths entries into an existing OpenCode
// configuration JSON string. If existingConfig is nil or empty, a new config
// object is created. Existing skills.paths entries are preserved (appended, deduplicated).
// Other fields in the config (including skills.urls) are preserved.
func injectSkillsIntoConfig(existingConfig *string, skillPaths []string) (*string, error) {
	if len(skillPaths) == 0 {
		return existingConfig, nil
	}

	// Parse existing config or start fresh
	configMap := make(map[string]interface{})
	if existingConfig != nil && *existingConfig != "" {
		if err := json.Unmarshal([]byte(*existingConfig), &configMap); err != nil {
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

	resultStr := string(result)
	return &resultStr, nil
}
