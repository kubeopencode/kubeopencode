// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FilterOptions holds parsed filter parameters from HTTP request
type FilterOptions struct {
	Name          string
	LabelSelector labels.Selector
	Limit         int
	Offset        int
	SortOrder     string
}

// ParseFilterOptions extracts filter options from query params
func ParseFilterOptions(r *http.Request) (*FilterOptions, error) {
	opts := &FilterOptions{
		Limit:     20,
		Offset:    0,
		SortOrder: "desc",
	}

	// Parse name filter
	opts.Name = r.URL.Query().Get("name")

	// Parse label selector (format: key1=value1,key2=value2)
	if labelStr := r.URL.Query().Get("labelSelector"); labelStr != "" {
		selector, err := labels.Parse(labelStr)
		if err != nil {
			return nil, err
		}
		opts.LabelSelector = selector
	}

	// Parse pagination
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			opts.Limit = min(parsed, 100)
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			opts.Offset = parsed
		}
	}

	if so := r.URL.Query().Get("sortOrder"); so == "asc" || so == "desc" {
		opts.SortOrder = so
	}

	return opts, nil
}

// BuildListOptions converts FilterOptions to Kubernetes client.ListOption slice
func BuildListOptions(namespace string, opts *FilterOptions) []client.ListOption {
	listOpts := []client.ListOption{}

	if namespace != "" {
		listOpts = append(listOpts, client.InNamespace(namespace))
	}

	if opts != nil && opts.LabelSelector != nil && !opts.LabelSelector.Empty() {
		listOpts = append(listOpts, client.MatchingLabelsSelector{
			Selector: opts.LabelSelector,
		})
	}

	return listOpts
}

// MatchesNameFilter checks if a resource name matches the name filter (case-insensitive substring)
func MatchesNameFilter(name, filter string) bool {
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(name), strings.ToLower(filter))
}
