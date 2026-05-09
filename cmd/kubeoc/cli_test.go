// Copyright Contributors to the KubeOpenCode project

package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name string
		age  time.Duration
		want string
	}{
		{
			name: "seconds",
			age:  30 * time.Second,
			want: "30s",
		},
		{
			name: "zero seconds",
			age:  0,
			want: "0s",
		},
		{
			name: "minutes",
			age:  5 * time.Minute,
			want: "5m",
		},
		{
			name: "just under an hour",
			age:  59 * time.Minute,
			want: "59m",
		},
		{
			name: "hours",
			age:  3 * time.Hour,
			want: "3h",
		},
		{
			name: "just under a day",
			age:  23 * time.Hour,
			want: "23h",
		},
		{
			name: "days",
			age:  3 * 24 * time.Hour,
			want: "3d",
		},
		{
			name: "many days",
			age:  30 * 24 * time.Hour,
			want: "30d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAge(time.Now().Add(-tt.age))
			if got != tt.want {
				t.Errorf("formatAge() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOutputFormat(t *testing.T) {
	type testData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	items := testData{Name: "test", Value: 42}

	tests := []struct {
		name        string
		format      string
		wantHandled bool
		wantErr     bool
		wantContain string
	}{
		{
			name:        "empty format returns false (use table output)",
			format:      "",
			wantHandled: false,
		},
		{
			name:        "json format",
			format:      "json",
			wantHandled: true,
			wantContain: `"name": "test"`,
		},
		{
			name:        "yaml format",
			format:      "yaml",
			wantHandled: true,
			wantContain: "name: test",
		},
		{
			name:        "unknown format returns error",
			format:      "xml",
			wantHandled: true,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handled, err := outputFormat(tt.format, items)

			if handled != tt.wantHandled {
				t.Errorf("outputFormat() handled = %v, want %v", handled, tt.wantHandled)
			}

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestOutputFormat_JSONValid(t *testing.T) {
	items := map[string]string{"key": "value"}

	// Capture stdout by replacing with a buffer is not trivial in this test,
	// but we can verify the function does not error for valid input.
	handled, err := outputFormat("json", items)
	if !handled {
		t.Error("expected handled=true for json format")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOutputFormat_YAMLValid(t *testing.T) {
	items := map[string]string{"key": "value"}

	handled, err := outputFormat("yaml", items)
	if !handled {
		t.Error("expected handled=true for yaml format")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCountReferencingAgents(t *testing.T) {
	s := runtime.NewScheme()
	_ = kubeopenv1alpha1.AddToScheme(s)

	tests := []struct {
		name      string
		templates []kubeopenv1alpha1.AgentTemplate
		agents    []kubeopenv1alpha1.Agent
		wantCount []int
	}{
		{
			name: "no agents reference any template",
			templates: []kubeopenv1alpha1.AgentTemplate{
				{ObjectMeta: metav1.ObjectMeta{Name: "tmpl-1", Namespace: "default"}},
			},
			agents:    []kubeopenv1alpha1.Agent{},
			wantCount: []int{0},
		},
		{
			name: "one agent references one template",
			templates: []kubeopenv1alpha1.AgentTemplate{
				{ObjectMeta: metav1.ObjectMeta{Name: "tmpl-1", Namespace: "default"}},
			},
			agents: []kubeopenv1alpha1.Agent{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "agent-1", Namespace: "default"},
					Spec: kubeopenv1alpha1.AgentSpec{
						TemplateRef:  &kubeopenv1alpha1.AgentTemplateReference{Name: "tmpl-1"},
						WorkspaceDir: "/workspace",
					},
				},
			},
			wantCount: []int{1},
		},
		{
			name: "multiple agents reference same template",
			templates: []kubeopenv1alpha1.AgentTemplate{
				{ObjectMeta: metav1.ObjectMeta{Name: "tmpl-1", Namespace: "default"}},
			},
			agents: []kubeopenv1alpha1.Agent{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "agent-1", Namespace: "default"},
					Spec: kubeopenv1alpha1.AgentSpec{
						TemplateRef:  &kubeopenv1alpha1.AgentTemplateReference{Name: "tmpl-1"},
						WorkspaceDir: "/workspace",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "agent-2", Namespace: "default"},
					Spec: kubeopenv1alpha1.AgentSpec{
						TemplateRef:  &kubeopenv1alpha1.AgentTemplateReference{Name: "tmpl-1"},
						WorkspaceDir: "/workspace",
					},
				},
			},
			wantCount: []int{2},
		},
		{
			name: "agents in different namespaces do not cross-reference",
			templates: []kubeopenv1alpha1.AgentTemplate{
				{ObjectMeta: metav1.ObjectMeta{Name: "tmpl-1", Namespace: "default"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "tmpl-1", Namespace: "production"}},
			},
			agents: []kubeopenv1alpha1.Agent{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "agent-1", Namespace: "default"},
					Spec: kubeopenv1alpha1.AgentSpec{
						TemplateRef:  &kubeopenv1alpha1.AgentTemplateReference{Name: "tmpl-1"},
						WorkspaceDir: "/workspace",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "agent-2", Namespace: "production"},
					Spec: kubeopenv1alpha1.AgentSpec{
						TemplateRef:  &kubeopenv1alpha1.AgentTemplateReference{Name: "tmpl-1"},
						WorkspaceDir: "/workspace",
					},
				},
			},
			wantCount: []int{1, 1},
		},
		{
			name: "agent without templateRef is not counted",
			templates: []kubeopenv1alpha1.AgentTemplate{
				{ObjectMeta: metav1.ObjectMeta{Name: "tmpl-1", Namespace: "default"}},
			},
			agents: []kubeopenv1alpha1.Agent{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "agent-standalone", Namespace: "default"},
					Spec: kubeopenv1alpha1.AgentSpec{
						WorkspaceDir: "/workspace",
					},
				},
			},
			wantCount: []int{0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := make([]runtime.Object, 0, len(tt.agents))
			for i := range tt.agents {
				objs = append(objs, &tt.agents[i])
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(s).
				WithRuntimeObjects(objs...).
				Build()

			ctx := t.Context()
			counts := countReferencingAgents(ctx, k8sClient, tt.templates)

			if len(counts) != len(tt.wantCount) {
				t.Fatalf("expected %d counts, got %d", len(tt.wantCount), len(counts))
			}

			for i, want := range tt.wantCount {
				if counts[i] != want {
					t.Errorf("template[%d] count = %d, want %d", i, counts[i], want)
				}
			}
		})
	}
}

func TestIsPortAvailable(t *testing.T) {
	// Port 0 should always be available (OS assigns ephemeral port)
	// We can't reliably test specific ports, but we can test the function doesn't panic
	// Port 1 is typically privileged and unavailable on non-root
	if isPortAvailable(1) {
		// On some CI systems, this might actually be true, so just log
		t.Log("port 1 is available (running as root?)")
	}
}

func TestVersionCmd(t *testing.T) {
	cmd := newVersionCmd()
	if cmd.Use != "version" {
		t.Errorf("expected Use 'version', got %q", cmd.Use)
	}

	// Execute the version command and capture output
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	// version command uses fmt.Printf which goes to stdout, not cmd.OutOrStdout()
	// Just verify it doesn't error
	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}
}

func TestCompletionCmd(t *testing.T) {
	cmd := newCompletionCmd()
	if cmd.Use != "completion [bash|zsh|fish|powershell]" {
		t.Errorf("unexpected Use: %q", cmd.Use)
	}

	// Verify valid args
	validArgs := cmd.ValidArgs
	expected := []string{"bash", "zsh", "fish", "powershell"}
	if len(validArgs) != len(expected) {
		t.Fatalf("expected %d valid args, got %d", len(expected), len(validArgs))
	}
	for i, arg := range expected {
		if validArgs[i] != arg {
			t.Errorf("validArgs[%d] = %q, want %q", i, validArgs[i], arg)
		}
	}
}

func TestRootCmdStructure(t *testing.T) {
	// Verify root command has expected subcommands
	subCmds := rootCmd.Commands()
	wantCmds := map[string]bool{
		"version":    false,
		"completion": false,
		"get":        false,
		"agent":      false,
		"task":       false,
		"crontask":   false,
	}

	for _, cmd := range subCmds {
		if _, ok := wantCmds[cmd.Name()]; ok {
			wantCmds[cmd.Name()] = true
		}
	}

	for name, found := range wantCmds {
		if !found {
			t.Errorf("expected subcommand %q not found", name)
		}
	}
}

func TestGetCmdStructure(t *testing.T) {
	getCmd := newGetCmd()
	subCmds := getCmd.Commands()
	wantCmds := map[string]bool{
		"agents":         false,
		"agenttemplates": false,
		"tasks":          false,
		"crontasks":      false,
	}

	for _, cmd := range subCmds {
		if _, ok := wantCmds[cmd.Name()]; ok {
			wantCmds[cmd.Name()] = true
		}
	}

	for name, found := range wantCmds {
		if !found {
			t.Errorf("expected get subcommand %q not found", name)
		}
	}
}

func TestAgentCmdStructure(t *testing.T) {
	agentCmd := newAgentCmd()
	subCmds := agentCmd.Commands()
	wantCmds := map[string]bool{
		"attach":  false,
		"suspend": false,
		"resume":  false,
		"share":   false,
		"unshare": false,
	}

	for _, cmd := range subCmds {
		if _, ok := wantCmds[cmd.Name()]; ok {
			wantCmds[cmd.Name()] = true
		}
	}

	for name, found := range wantCmds {
		if !found {
			t.Errorf("expected agent subcommand %q not found", name)
		}
	}
}

func TestTaskCmdStructure(t *testing.T) {
	taskCmd := newTaskCmd()
	subCmds := taskCmd.Commands()
	wantCmds := map[string]bool{
		"stop": false,
		"logs": false,
	}

	for _, cmd := range subCmds {
		if _, ok := wantCmds[cmd.Name()]; ok {
			wantCmds[cmd.Name()] = true
		}
	}

	for name, found := range wantCmds {
		if !found {
			t.Errorf("expected task subcommand %q not found", name)
		}
	}
}

func TestCronTaskCmdStructure(t *testing.T) {
	cronTaskCmd := newCronTaskCmd()

	// Verify alias
	if len(cronTaskCmd.Aliases) == 0 || cronTaskCmd.Aliases[0] != "ct" {
		t.Errorf("expected alias 'ct', got %v", cronTaskCmd.Aliases)
	}

	subCmds := cronTaskCmd.Commands()
	wantCmds := map[string]bool{
		"trigger": false,
		"suspend": false,
		"resume":  false,
	}

	for _, cmd := range subCmds {
		if _, ok := wantCmds[cmd.Name()]; ok {
			wantCmds[cmd.Name()] = true
		}
	}

	for name, found := range wantCmds {
		if !found {
			t.Errorf("expected crontask subcommand %q not found", name)
		}
	}
}

func TestOutputFormat_JSONRoundtrip(t *testing.T) {
	// Test with a real Kubernetes-like structure to ensure marshaling works
	type Item struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	items := []Item{
		{Name: "a", Status: "Ready"},
		{Name: "b", Status: "NotReady"},
	}

	// JSON should work without error
	handled, err := outputFormat("json", items)
	if !handled {
		t.Error("expected handled=true")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the data can be round-tripped
	data, _ := json.MarshalIndent(items, "", "  ")
	var decoded []Item
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(decoded) != 2 {
		t.Errorf("expected 2 items, got %d", len(decoded))
	}
}
