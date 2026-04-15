// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"encoding/json"
	"testing"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestProcessSkills(t *testing.T) {
	t.Run("empty skills returns nil", func(t *testing.T) {
		gitMounts, skillPaths := processSkills(nil)
		if gitMounts != nil {
			t.Errorf("expected nil gitMounts, got %v", gitMounts)
		}
		if skillPaths != nil {
			t.Errorf("expected nil skillPaths, got %v", skillPaths)
		}
	})

	t.Run("single skill without names filter", func(t *testing.T) {
		depth := 1
		skills := []kubeopenv1alpha1.SkillSource{
			{
				Name: "my-skills",
				Git: &kubeopenv1alpha1.GitSkillSource{
					Repository: "https://github.com/anthropics/skills.git",
					Ref:        "main",
					Path:       "skills/",
					Depth:      &depth,
				},
			},
		}

		gitMounts, skillPaths := processSkills(skills)

		if len(gitMounts) != 1 {
			t.Fatalf("expected 1 gitMount, got %d", len(gitMounts))
		}
		gm := gitMounts[0]
		if gm.contextName != "skill-my-skills" {
			t.Errorf("contextName = %q, want %q", gm.contextName, "skill-my-skills")
		}
		if gm.repository != "https://github.com/anthropics/skills.git" {
			t.Errorf("repository = %q", gm.repository)
		}
		if gm.ref != "main" {
			t.Errorf("ref = %q, want %q", gm.ref, "main")
		}
		if gm.repoPath != "skills/" {
			t.Errorf("repoPath = %q, want %q", gm.repoPath, "skills/")
		}
		if gm.mountPath != "/skills/my-skills" {
			t.Errorf("mountPath = %q, want %q", gm.mountPath, "/skills/my-skills")
		}
		if gm.depth != 1 {
			t.Errorf("depth = %d, want %d", gm.depth, 1)
		}

		if len(skillPaths) != 1 {
			t.Fatalf("expected 1 skillPath, got %d", len(skillPaths))
		}
		if skillPaths[0] != "/skills/my-skills" {
			t.Errorf("skillPath = %q, want %q", skillPaths[0], "/skills/my-skills")
		}
	})

	t.Run("skill with names filter", func(t *testing.T) {
		skills := []kubeopenv1alpha1.SkillSource{
			{
				Name: "official",
				Git: &kubeopenv1alpha1.GitSkillSource{
					Repository: "https://github.com/anthropics/skills.git",
					Path:       "skills/",
					Names:      []string{"frontend-design", "webapp-testing"},
				},
			},
		}

		gitMounts, skillPaths := processSkills(skills)

		if len(gitMounts) != 1 {
			t.Fatalf("expected 1 gitMount, got %d", len(gitMounts))
		}

		// Verify names are passed through to gitMount for per-name SubPath mounting
		gm := gitMounts[0]
		if len(gm.names) != 2 {
			t.Fatalf("expected 2 names on gitMount, got %d", len(gm.names))
		}
		if gm.names[0] != "frontend-design" || gm.names[1] != "webapp-testing" {
			t.Errorf("names = %v, want [frontend-design webapp-testing]", gm.names)
		}

		if len(skillPaths) != 2 {
			t.Fatalf("expected 2 skillPaths, got %d", len(skillPaths))
		}
		if skillPaths[0] != "/skills/official/frontend-design" {
			t.Errorf("skillPath[0] = %q, want %q", skillPaths[0], "/skills/official/frontend-design")
		}
		if skillPaths[1] != "/skills/official/webapp-testing" {
			t.Errorf("skillPath[1] = %q, want %q", skillPaths[1], "/skills/official/webapp-testing")
		}
	})

	t.Run("multiple skills", func(t *testing.T) {
		skills := []kubeopenv1alpha1.SkillSource{
			{
				Name: "source-a",
				Git: &kubeopenv1alpha1.GitSkillSource{
					Repository: "https://github.com/org/skills-a.git",
				},
			},
			{
				Name: "source-b",
				Git: &kubeopenv1alpha1.GitSkillSource{
					Repository: "https://github.com/org/skills-b.git",
					Names:      []string{"skill-x"},
				},
			},
		}

		gitMounts, skillPaths := processSkills(skills)

		if len(gitMounts) != 2 {
			t.Fatalf("expected 2 gitMounts, got %d", len(gitMounts))
		}
		if len(skillPaths) != 2 {
			t.Fatalf("expected 2 skillPaths, got %d", len(skillPaths))
		}
		if skillPaths[0] != "/skills/source-a" {
			t.Errorf("skillPath[0] = %q", skillPaths[0])
		}
		if skillPaths[1] != "/skills/source-b/skill-x" {
			t.Errorf("skillPath[1] = %q", skillPaths[1])
		}
	})

	t.Run("skill with secretRef", func(t *testing.T) {
		skills := []kubeopenv1alpha1.SkillSource{
			{
				Name: "private",
				Git: &kubeopenv1alpha1.GitSkillSource{
					Repository: "https://github.com/org/private-skills.git",
					SecretRef:  &kubeopenv1alpha1.GitSecretReference{Name: "git-creds"},
				},
			},
		}

		gitMounts, _ := processSkills(skills)

		if len(gitMounts) != 1 {
			t.Fatalf("expected 1 gitMount, got %d", len(gitMounts))
		}
		if gitMounts[0].secretName != "git-creds" {
			t.Errorf("secretName = %q, want %q", gitMounts[0].secretName, "git-creds")
		}
	})

	t.Run("skill with nil git is skipped", func(t *testing.T) {
		skills := []kubeopenv1alpha1.SkillSource{
			{Name: "empty"},
		}

		gitMounts, skillPaths := processSkills(skills)

		if len(gitMounts) != 0 {
			t.Errorf("expected 0 gitMounts, got %d", len(gitMounts))
		}
		if len(skillPaths) != 0 {
			t.Errorf("expected 0 skillPaths, got %d", len(skillPaths))
		}
	})

	t.Run("default ref is HEAD", func(t *testing.T) {
		skills := []kubeopenv1alpha1.SkillSource{
			{
				Name: "test",
				Git: &kubeopenv1alpha1.GitSkillSource{
					Repository: "https://github.com/org/repo.git",
				},
			},
		}

		gitMounts, _ := processSkills(skills)
		if gitMounts[0].ref != "HEAD" {
			t.Errorf("ref = %q, want %q", gitMounts[0].ref, "HEAD")
		}
	})
}

func TestInjectSkillsIntoConfig(t *testing.T) {
	t.Run("nil config creates new config", func(t *testing.T) {
		result, err := injectSkillsIntoConfig(nil, []string{"/skills/a"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal(result.Raw, &parsed); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		skills := parsed["skills"].(map[string]interface{})
		paths := skills["paths"].([]interface{})
		if len(paths) != 1 || paths[0].(string) != "/skills/a" {
			t.Errorf("paths = %v, want [/skills/a]", paths)
		}
	})

	t.Run("empty config creates new config", func(t *testing.T) {
		empty := &runtime.RawExtension{Raw: []byte("")}
		result, err := injectSkillsIntoConfig(empty, []string{"/skills/a"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal(result.Raw, &parsed); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		skills := parsed["skills"].(map[string]interface{})
		paths := skills["paths"].([]interface{})
		if len(paths) != 1 {
			t.Errorf("expected 1 path, got %d", len(paths))
		}
	})

	t.Run("preserves existing config fields", func(t *testing.T) {
		existing := &runtime.RawExtension{Raw: []byte(`{"model":"claude","someField":"value"}`)}
		result, err := injectSkillsIntoConfig(existing, []string{"/skills/a"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal(result.Raw, &parsed); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		if parsed["model"] != "claude" {
			t.Errorf("model field lost: %v", parsed)
		}
		if parsed["someField"] != "value" {
			t.Errorf("someField lost: %v", parsed)
		}
	})

	t.Run("appends to existing skills.paths", func(t *testing.T) {
		existing := &runtime.RawExtension{Raw: []byte(`{"skills":{"paths":["/existing/path"]}}`)}
		result, err := injectSkillsIntoConfig(existing, []string{"/skills/new"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal(result.Raw, &parsed); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		skills := parsed["skills"].(map[string]interface{})
		paths := skills["paths"].([]interface{})
		if len(paths) != 2 {
			t.Errorf("expected 2 paths, got %d: %v", len(paths), paths)
		}
	})

	t.Run("deduplicates paths", func(t *testing.T) {
		existing := &runtime.RawExtension{Raw: []byte(`{"skills":{"paths":["/skills/a"]}}`)}
		result, err := injectSkillsIntoConfig(existing, []string{"/skills/a", "/skills/b"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal(result.Raw, &parsed); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		skills := parsed["skills"].(map[string]interface{})
		paths := skills["paths"].([]interface{})
		if len(paths) != 2 {
			t.Errorf("expected 2 paths (deduped), got %d: %v", len(paths), paths)
		}
	})

	t.Run("preserves skills.urls", func(t *testing.T) {
		existing := &runtime.RawExtension{Raw: []byte(`{"skills":{"urls":["https://example.com/skills/"]}}`)}
		result, err := injectSkillsIntoConfig(existing, []string{"/skills/a"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal(result.Raw, &parsed); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		skills := parsed["skills"].(map[string]interface{})
		urls := skills["urls"].([]interface{})
		if len(urls) != 1 || urls[0].(string) != "https://example.com/skills/" {
			t.Errorf("urls lost: %v", urls)
		}
	})

	t.Run("empty skillPaths returns original config", func(t *testing.T) {
		existing := &runtime.RawExtension{Raw: []byte(`{"model":"claude"}`)}
		result, err := injectSkillsIntoConfig(existing, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Raw) != `{"model":"claude"}` {
			t.Errorf("expected original config, got %q", string(result.Raw))
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		invalid := &runtime.RawExtension{Raw: []byte(`{invalid`)}
		_, err := injectSkillsIntoConfig(invalid, []string{"/skills/a"})
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestInjectPluginsIntoConfig(t *testing.T) {
	t.Run("nil config creates new config with plugins", func(t *testing.T) {
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "@kubeopencode/opencode-slack-plugin"},
		}
		result, err := injectPluginsIntoConfig(nil, plugins)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(result.Raw, &parsed); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		pluginArr, ok := parsed["plugin"].([]interface{})
		if !ok {
			t.Fatal("expected plugin array in config")
		}
		if len(pluginArr) != 1 {
			t.Fatalf("expected 1 plugin, got %d", len(pluginArr))
		}
		if pluginArr[0] != "@kubeopencode/opencode-slack-plugin" {
			t.Errorf("expected plugin name, got %v", pluginArr[0])
		}
	})

	t.Run("empty config creates new config", func(t *testing.T) {
		empty := &runtime.RawExtension{Raw: []byte("")}
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "opencode-plugin-otel"},
		}
		result, err := injectPluginsIntoConfig(empty, plugins)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(result.Raw, &parsed); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		pluginArr := parsed["plugin"].([]interface{})
		if len(pluginArr) != 1 || pluginArr[0] != "opencode-plugin-otel" {
			t.Errorf("unexpected plugins: %v", pluginArr)
		}
	})

	t.Run("preserves existing config fields", func(t *testing.T) {
		existing := &runtime.RawExtension{Raw: []byte(`{"model":"claude-sonnet","provider":{"anthropic":{}}}`)}
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "@org/my-plugin"},
		}
		result, err := injectPluginsIntoConfig(existing, plugins)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(result.Raw, &parsed); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		if parsed["model"] != "claude-sonnet" {
			t.Errorf("model field lost: %v", parsed["model"])
		}
		if parsed["provider"] == nil {
			t.Error("provider field lost")
		}
		pluginArr := parsed["plugin"].([]interface{})
		if len(pluginArr) != 1 {
			t.Fatalf("expected 1 plugin, got %d", len(pluginArr))
		}
	})

	t.Run("appends to existing plugins", func(t *testing.T) {
		existing := &runtime.RawExtension{Raw: []byte(`{"plugin":["existing-plugin"]}`)}
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "new-plugin"},
		}
		result, err := injectPluginsIntoConfig(existing, plugins)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(result.Raw, &parsed); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		pluginArr := parsed["plugin"].([]interface{})
		if len(pluginArr) != 2 {
			t.Fatalf("expected 2 plugins, got %d", len(pluginArr))
		}
		if pluginArr[0] != "existing-plugin" {
			t.Errorf("first plugin should be existing, got %v", pluginArr[0])
		}
		if pluginArr[1] != "new-plugin" {
			t.Errorf("second plugin should be new, got %v", pluginArr[1])
		}
	})

	t.Run("deduplicates by name", func(t *testing.T) {
		existing := &runtime.RawExtension{Raw: []byte(`{"plugin":["@org/plugin-a"]}`)}
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "@org/plugin-a"}, // duplicate
			{Name: "@org/plugin-b"}, // new
		}
		result, err := injectPluginsIntoConfig(existing, plugins)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(result.Raw, &parsed); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		pluginArr := parsed["plugin"].([]interface{})
		if len(pluginArr) != 2 {
			t.Fatalf("expected 2 plugins (1 existing + 1 new), got %d", len(pluginArr))
		}
	})

	t.Run("deduplicates tuple format existing plugins", func(t *testing.T) {
		existing := &runtime.RawExtension{Raw: []byte(`{"plugin":[["@org/plugin-a",{"key":"val"}]]}`)}
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "@org/plugin-a"}, // duplicate (existing is tuple)
		}
		result, err := injectPluginsIntoConfig(existing, plugins)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should return original since no new plugins added
		if string(result.Raw) != string(existing.Raw) {
			t.Errorf("expected original config returned unchanged, got %v", string(result.Raw))
		}
	})

	t.Run("plugin with options creates tuple format", func(t *testing.T) {
		plugins := []kubeopenv1alpha1.PluginSpec{
			{
				Name:    "opencode-plugin-otel",
				Options: &runtime.RawExtension{Raw: []byte(`{"endpoint":"http://collector:4318","verbose":true}`)},
			},
		}
		result, err := injectPluginsIntoConfig(nil, plugins)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(result.Raw, &parsed); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		pluginArr := parsed["plugin"].([]interface{})
		if len(pluginArr) != 1 {
			t.Fatalf("expected 1 plugin, got %d", len(pluginArr))
		}
		// Should be a tuple: ["opencode-plugin-otel", { "endpoint": "...", "verbose": true }]
		tuple, ok := pluginArr[0].([]interface{})
		if !ok {
			t.Fatalf("expected tuple format, got %T: %v", pluginArr[0], pluginArr[0])
		}
		if len(tuple) != 2 {
			t.Fatalf("expected tuple of length 2, got %d", len(tuple))
		}
		if tuple[0] != "opencode-plugin-otel" {
			t.Errorf("expected plugin name, got %v", tuple[0])
		}
		opts, ok := tuple[1].(map[string]interface{})
		if !ok {
			t.Fatalf("expected options map, got %T", tuple[1])
		}
		if opts["endpoint"] != "http://collector:4318" {
			t.Errorf("expected endpoint option, got %v", opts["endpoint"])
		}
	})

	t.Run("mixed plugins with and without options", func(t *testing.T) {
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "@kubeopencode/opencode-slack-plugin"},
			{
				Name:    "opencode-plugin-otel",
				Options: &runtime.RawExtension{Raw: []byte(`{"endpoint":"http://collector:4318"}`)},
			},
		}
		result, err := injectPluginsIntoConfig(nil, plugins)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(result.Raw, &parsed); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		pluginArr := parsed["plugin"].([]interface{})
		if len(pluginArr) != 2 {
			t.Fatalf("expected 2 plugins, got %d", len(pluginArr))
		}
		// First should be string
		if pluginArr[0] != "@kubeopencode/opencode-slack-plugin" {
			t.Errorf("first plugin should be string, got %v", pluginArr[0])
		}
		// Second should be tuple
		tuple, ok := pluginArr[1].([]interface{})
		if !ok {
			t.Fatalf("second plugin should be tuple, got %T", pluginArr[1])
		}
		if tuple[0] != "opencode-plugin-otel" {
			t.Errorf("expected plugin name in tuple, got %v", tuple[0])
		}
	})

	t.Run("empty plugins returns original config", func(t *testing.T) {
		existing := &runtime.RawExtension{Raw: []byte(`{"model":"claude"}`)}
		result, err := injectPluginsIntoConfig(existing, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Raw) != string(existing.Raw) {
			t.Errorf("expected original config, got %v", string(result.Raw))
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		invalid := &runtime.RawExtension{Raw: []byte(`{invalid`)}
		_, err := injectPluginsIntoConfig(invalid, []kubeopenv1alpha1.PluginSpec{
			{Name: "test-plugin"},
		})
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("invalid plugin options JSON returns error", func(t *testing.T) {
		plugins := []kubeopenv1alpha1.PluginSpec{
			{
				Name:    "test-plugin",
				Options: &runtime.RawExtension{Raw: []byte(`{invalid}`)},
			},
		}
		_, err := injectPluginsIntoConfig(nil, plugins)
		if err == nil {
			t.Fatal("expected error for invalid plugin options JSON")
		}
	})
}

func TestSplitPluginsByTarget(t *testing.T) {
	t.Run("empty plugins returns nil slices", func(t *testing.T) {
		server, tui := splitPluginsByTarget(nil)
		if server != nil || tui != nil {
			t.Errorf("expected nil slices, got server=%v, tui=%v", server, tui)
		}
	})

	t.Run("all server plugins (default target)", func(t *testing.T) {
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "plugin-a"},
			{Name: "plugin-b", Target: kubeopenv1alpha1.PluginTargetServer},
		}
		server, tui := splitPluginsByTarget(plugins)
		if len(server) != 2 {
			t.Errorf("expected 2 server plugins, got %d", len(server))
		}
		if tui != nil {
			t.Errorf("expected nil tui plugins, got %v", tui)
		}
	})

	t.Run("all TUI plugins", func(t *testing.T) {
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "tui-plugin-a", Target: kubeopenv1alpha1.PluginTargetTUI},
			{Name: "tui-plugin-b", Target: kubeopenv1alpha1.PluginTargetTUI},
		}
		server, tui := splitPluginsByTarget(plugins)
		if server != nil {
			t.Errorf("expected nil server plugins, got %v", server)
		}
		if len(tui) != 2 {
			t.Errorf("expected 2 tui plugins, got %d", len(tui))
		}
	})

	t.Run("mixed server and TUI plugins", func(t *testing.T) {
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "server-plugin", Target: kubeopenv1alpha1.PluginTargetServer},
			{Name: "tui-theme", Target: kubeopenv1alpha1.PluginTargetTUI},
			{Name: "default-plugin"}, // empty target defaults to server
			{Name: "tui-commands", Target: kubeopenv1alpha1.PluginTargetTUI},
		}
		server, tui := splitPluginsByTarget(plugins)
		if len(server) != 2 {
			t.Errorf("expected 2 server plugins, got %d", len(server))
		}
		if server[0].Name != "server-plugin" || server[1].Name != "default-plugin" {
			t.Errorf("unexpected server plugins: %v", server)
		}
		if len(tui) != 2 {
			t.Errorf("expected 2 tui plugins, got %d", len(tui))
		}
		if tui[0].Name != "tui-theme" || tui[1].Name != "tui-commands" {
			t.Errorf("unexpected tui plugins: %v", tui)
		}
	})
}

func TestHasServerPlugins(t *testing.T) {
	if hasServerPlugins(nil) {
		t.Error("nil plugins should return false")
	}
	if !hasServerPlugins([]kubeopenv1alpha1.PluginSpec{{Name: "a"}}) {
		t.Error("default target plugin should count as server")
	}
	if !hasServerPlugins([]kubeopenv1alpha1.PluginSpec{{Name: "a", Target: kubeopenv1alpha1.PluginTargetServer}}) {
		t.Error("explicit server target should count")
	}
	if hasServerPlugins([]kubeopenv1alpha1.PluginSpec{{Name: "a", Target: kubeopenv1alpha1.PluginTargetTUI}}) {
		t.Error("only TUI plugins should return false")
	}
}

func TestHasTUIPlugins(t *testing.T) {
	if hasTUIPlugins(nil) {
		t.Error("nil plugins should return false")
	}
	if hasTUIPlugins([]kubeopenv1alpha1.PluginSpec{{Name: "a"}}) {
		t.Error("default target plugin should not count as TUI")
	}
	if !hasTUIPlugins([]kubeopenv1alpha1.PluginSpec{{Name: "a", Target: kubeopenv1alpha1.PluginTargetTUI}}) {
		t.Error("TUI target plugin should count")
	}
}

func TestResolvePluginPaths(t *testing.T) {
	t.Run("nil plugins returns nil", func(t *testing.T) {
		resolved := resolvePluginPaths(nil)
		if resolved != nil {
			t.Errorf("expected nil, got %v", resolved)
		}
	})

	t.Run("unscoped package rewrites to file path", func(t *testing.T) {
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "cc-safety-net"},
		}
		resolved := resolvePluginPaths(plugins)
		if len(resolved) != 1 {
			t.Fatalf("expected 1 resolved plugin, got %d", len(resolved))
		}
		if resolved[0].Name != "file:///plugins/node_modules/cc-safety-net" {
			t.Errorf("expected file:// path, got %v", resolved[0].Name)
		}
	})

	t.Run("scoped package preserves scope in path", func(t *testing.T) {
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "@aexol/opencode-tui"},
		}
		resolved := resolvePluginPaths(plugins)
		if resolved[0].Name != "file:///plugins/node_modules/@aexol/opencode-tui" {
			t.Errorf("expected scoped path, got %v", resolved[0].Name)
		}
	})

	t.Run("version specifier stripped from path", func(t *testing.T) {
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "cc-safety-net@0.8.2"},
		}
		resolved := resolvePluginPaths(plugins)
		if resolved[0].Name != "file:///plugins/node_modules/cc-safety-net" {
			t.Errorf("expected version stripped, got %v", resolved[0].Name)
		}
	})

	t.Run("scoped package with version", func(t *testing.T) {
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "@org/plugin@^1.0.0"},
		}
		resolved := resolvePluginPaths(plugins)
		if resolved[0].Name != "file:///plugins/node_modules/@org/plugin" {
			t.Errorf("expected scoped path without version, got %v", resolved[0].Name)
		}
	})

	t.Run("duplicates deduplicated by package name", func(t *testing.T) {
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "cc-safety-net@0.8.2"},
			{Name: "cc-safety-net@0.9.0"}, // same package, different version
		}
		resolved := resolvePluginPaths(plugins)
		if len(resolved) != 1 {
			t.Errorf("expected 1 plugin (deduped), got %d", len(resolved))
		}
	})

	t.Run("target preserved", func(t *testing.T) {
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "server-plugin", Target: kubeopenv1alpha1.PluginTargetServer},
			{Name: "@org/tui-plugin", Target: kubeopenv1alpha1.PluginTargetTUI},
		}
		resolved := resolvePluginPaths(plugins)
		if len(resolved) != 2 {
			t.Fatalf("expected 2, got %d", len(resolved))
		}
		if resolved[0].Target != kubeopenv1alpha1.PluginTargetServer {
			t.Errorf("server target not preserved")
		}
		if resolved[1].Target != kubeopenv1alpha1.PluginTargetTUI {
			t.Errorf("tui target not preserved")
		}
	})

	t.Run("options preserved", func(t *testing.T) {
		opts := &runtime.RawExtension{Raw: []byte(`{"key":"value"}`)}
		plugins := []kubeopenv1alpha1.PluginSpec{
			{Name: "my-plugin", Options: opts},
		}
		resolved := resolvePluginPaths(plugins)
		if resolved[0].Options == nil || string(resolved[0].Options.Raw) != `{"key":"value"}` {
			t.Errorf("options should be preserved, got %v", resolved[0].Options)
		}
	})
}

func TestExtractNpmPackageName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"cc-safety-net", "cc-safety-net"},
		{"cc-safety-net@0.8.2", "cc-safety-net"},
		{"cc-safety-net@^1.0.0", "cc-safety-net"},
		{"@org/plugin", "@org/plugin"},
		{"@org/plugin@1.2.0", "@org/plugin"},
		{"@org/plugin@^1.0.0", "@org/plugin"},
		{"@aexol/opencode-tui", "@aexol/opencode-tui"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractNpmPackageName(tt.input)
			if got != tt.expected {
				t.Errorf("extractNpmPackageName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
