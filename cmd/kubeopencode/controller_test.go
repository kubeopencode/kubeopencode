// Copyright Contributors to the KubeOpenCode project

package main

import (
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func TestBuildCacheOptions(t *testing.T) {
	tests := []struct {
		name       string
		namespaces []string
		want       map[string]cache.Config
	}{
		{
			name:       "nil watches all namespaces",
			namespaces: nil,
			want:       nil,
		},
		{
			name:       "empty slice watches all namespaces",
			namespaces: []string{},
			want:       nil,
		},
		{
			name:       "blank entries are ignored and watch all namespaces",
			namespaces: []string{"", "   "},
			want:       nil,
		},
		{
			name:       "single namespace",
			namespaces: []string{"kubeopencode-system"},
			want:       map[string]cache.Config{"kubeopencode-system": {}},
		},
		{
			name:       "multiple namespaces",
			namespaces: []string{"team-a", "team-b"},
			want: map[string]cache.Config{
				"team-a": {},
				"team-b": {},
			},
		},
		{
			name:       "surrounding whitespace is trimmed",
			namespaces: []string{" team-a ", "team-b"},
			want: map[string]cache.Config{
				"team-a": {},
				"team-b": {},
			},
		},
		{
			name:       "duplicates collapse to one entry",
			namespaces: []string{"team-a", "team-a"},
			want:       map[string]cache.Config{"team-a": {}},
		},
		{
			name:       "blank entries are dropped but real ones are kept",
			namespaces: []string{"team-a", "", "team-b"},
			want: map[string]cache.Config{
				"team-a": {},
				"team-b": {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCacheOptions(tt.namespaces).DefaultNamespaces

			if len(got) != len(tt.want) {
				t.Fatalf("DefaultNamespaces has %d entries, want %d (got %v)",
					len(got), len(tt.want), got)
			}
			for ns := range tt.want {
				if _, ok := got[ns]; !ok {
					t.Errorf("DefaultNamespaces is missing namespace %q (got %v)", ns, got)
				}
			}
		})
	}
}
