// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

const (
	// defaultCheckInterval is the default interval for re-checking asset status.
	defaultCheckInterval = 10 * time.Minute

	// statusCheckTimeout is the max duration for a single asset status check.
	statusCheckTimeout = 30 * time.Second

	// maxConcurrentChecks limits the number of parallel HTTP status checks
	// to avoid overwhelming external services (npm registry, container registries).
	maxConcurrentChecks = 10
)

// RegistryReconciler reconciles Registry resources.
// It periodically checks the availability of registered images, skills, and plugins,
// updating status to reflect whether each asset is reachable.
type RegistryReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	HTTPClient *http.Client
}

// +kubebuilder:rbac:groups=kubeopencode.io,resources=registries,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=kubeopencode.io,resources=registries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubeopencode.io,resources=registries/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile handles Registry reconciliation.
func (r *RegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var registry kubeopenv1alpha1.Registry
	if err := r.Get(ctx, req.NamespacedName, &registry); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Registry")
		return ctrl.Result{}, err
	}

	// Run all status checks
	imageStatuses := r.checkImages(ctx, &registry)
	skillStatuses := r.checkSkills(ctx, &registry)
	pluginStatuses := r.checkPlugins(ctx, &registry)

	// Build summary
	readyCount := 0
	for i := range imageStatuses {
		if imageStatuses[i].Phase == kubeopenv1alpha1.AssetPhaseReady {
			readyCount++
		}
	}
	for i := range skillStatuses {
		if skillStatuses[i].Phase == kubeopenv1alpha1.AssetPhaseReady {
			readyCount++
		}
	}
	for i := range pluginStatuses {
		if pluginStatuses[i].Phase == kubeopenv1alpha1.AssetPhaseReady {
			readyCount++
		}
	}

	totalCount := len(imageStatuses) + len(skillStatuses) + len(pluginStatuses)

	// Update status
	registry.Status.ObservedGeneration = registry.Generation
	registry.Status.Images = imageStatuses
	registry.Status.Skills = skillStatuses
	registry.Status.Plugins = pluginStatuses
	registry.Status.Summary = kubeopenv1alpha1.StatusSummary{
		Images:     len(imageStatuses),
		Skills:     len(skillStatuses),
		Plugins:    len(pluginStatuses),
		ReadyCount: readyCount,
		TotalCount: totalCount,
	}

	// Set overall Ready condition
	switch {
	case readyCount == totalCount && totalCount > 0:
		meta.SetStatusCondition(&registry.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: registry.Generation,
			Reason:             "AllAssetsReady",
			Message:            fmt.Sprintf("All %d assets are ready", totalCount),
		})
	case totalCount == 0:
		meta.SetStatusCondition(&registry.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: registry.Generation,
			Reason:             "Empty",
			Message:            "Registry has no assets defined",
		})
	default:
		meta.SetStatusCondition(&registry.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: registry.Generation,
			Reason:             "AssetsUnavailable",
			Message:            fmt.Sprintf("%d of %d assets are ready", readyCount, totalCount),
		})
	}

	if err := r.Status().Update(ctx, &registry); err != nil {
		logger.Error(err, "Failed to update Registry status")
		return ctrl.Result{}, err
	}

	// Requeue after checkInterval for periodic re-checks
	requeueAfter := defaultCheckInterval
	if registry.Spec.CheckInterval != nil {
		requeueAfter = registry.Spec.CheckInterval.Duration
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// checkImages validates each registered image in parallel.
// For Phase 1, we validate the image reference format.
// Full registry API (HEAD manifest) checks will be added when go-containerregistry
// is added as a dependency.
func (r *RegistryReconciler) checkImages(ctx context.Context, registry *kubeopenv1alpha1.Registry) []kubeopenv1alpha1.ImageStatus {
	if len(registry.Spec.Images) == 0 {
		return nil
	}

	statuses := make([]kubeopenv1alpha1.ImageStatus, len(registry.Spec.Images))
	sem := make(chan struct{}, maxConcurrentChecks)

	var wg sync.WaitGroup
	for i, img := range registry.Spec.Images {
		wg.Add(1)
		go func(idx int, image kubeopenv1alpha1.RegistryImage) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			checkCtx, cancel := context.WithTimeout(ctx, statusCheckTimeout)
			defer cancel()

			now := metav1.Now()
			status := kubeopenv1alpha1.ImageStatus{
				Name:        image.Name,
				Image:       image.Image,
				LastChecked: &now,
			}

			// Validate image reference format
			if strings.TrimSpace(image.Image) == "" {
				status.Phase = kubeopenv1alpha1.AssetPhaseUnavailable
				status.Message = "image reference is empty"
				statuses[idx] = status
				return
			}

			// Check if the image is reachable via registry API
			digest, err := r.checkImageExists(checkCtx, image)
			if err != nil {
				status.Phase = kubeopenv1alpha1.AssetPhaseUnavailable
				status.Message = err.Error()
			} else {
				status.Phase = kubeopenv1alpha1.AssetPhaseReady
				status.Digest = digest
			}

			statuses[idx] = status
		}(i, img)
	}
	wg.Wait()

	return statuses
}

// checkImageExists validates the image reference format.
// Phase 1: basic format validation only (non-empty string).
// A full implementation will use go-containerregistry's remote.Head()
// to verify the image exists in the registry via a manifest HEAD request.
func (r *RegistryReconciler) checkImageExists(_ context.Context, image kubeopenv1alpha1.RegistryImage) (string, error) {
	ref := image.Image

	// Accept any non-empty reference. Docker Hub shorthand like "ubuntu" or
	// "nginx" is valid (resolves to docker.io/library/ubuntu:latest), as are
	// fully qualified references like "harbor.company.com/team/go-dev:1.23".
	// Real validation will come with go-containerregistry which handles all
	// reference formats including digest references.
	if strings.TrimSpace(ref) == "" {
		return "", fmt.Errorf("image reference is empty")
	}

	// Mark as ready with empty digest (will be populated when go-containerregistry is added)
	return "", nil
}

// checkSkills validates each registered skill source in parallel.
func (r *RegistryReconciler) checkSkills(ctx context.Context, registry *kubeopenv1alpha1.Registry) []kubeopenv1alpha1.SkillStatus {
	if len(registry.Spec.Skills) == 0 {
		return nil
	}

	statuses := make([]kubeopenv1alpha1.SkillStatus, len(registry.Spec.Skills))
	sem := make(chan struct{}, maxConcurrentChecks)

	var wg sync.WaitGroup
	for i, skill := range registry.Spec.Skills {
		wg.Add(1)
		go func(idx int, skill kubeopenv1alpha1.RegistrySkill) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			now := metav1.Now()
			status := kubeopenv1alpha1.SkillStatus{
				Name:        skill.Name,
				LastChecked: &now,
			}

			if skill.Git == nil {
				status.Phase = kubeopenv1alpha1.AssetPhaseUnavailable
				status.Message = "no git source specified"
				statuses[idx] = status
				return
			}

			if skill.Git.Repository == "" {
				status.Phase = kubeopenv1alpha1.AssetPhaseUnavailable
				status.Message = "git repository URL is empty"
				statuses[idx] = status
				return
			}

			// For Phase 1: validate the repository URL format.
			// Full go-git ls-remote integration will be added when go-git is added as a dependency.
			// For now, we mark skills with valid Git URLs as Ready.
			if !strings.HasPrefix(skill.Git.Repository, "https://") &&
				!strings.HasPrefix(skill.Git.Repository, "http://") &&
				!strings.HasPrefix(skill.Git.Repository, "git@") {
				status.Phase = kubeopenv1alpha1.AssetPhaseUnavailable
				status.Message = fmt.Sprintf("unsupported repository URL scheme: %s", skill.Git.Repository)
				statuses[idx] = status
				return
			}

			// Verify referenced Secret exists (lightweight check)
			if skill.Git.SecretRef != nil && skill.Git.SecretRef.Name != "" {
				var secret corev1.Secret
				secretKey := client.ObjectKey{
					Namespace: registry.Namespace,
					Name:      skill.Git.SecretRef.Name,
				}
				if err := r.Get(ctx, secretKey, &secret); err != nil {
					status.Phase = kubeopenv1alpha1.AssetPhaseUnavailable
					status.Message = fmt.Sprintf("git secret %q not found", skill.Git.SecretRef.Name)
					statuses[idx] = status
					return
				}
			}

			status.Phase = kubeopenv1alpha1.AssetPhaseReady
			statuses[idx] = status
		}(i, skill)
	}
	wg.Wait()

	return statuses
}

// checkPlugins validates each registered plugin in parallel.
func (r *RegistryReconciler) checkPlugins(ctx context.Context, registry *kubeopenv1alpha1.Registry) []kubeopenv1alpha1.PluginStatus {
	if len(registry.Spec.Plugins) == 0 {
		return nil
	}

	statuses := make([]kubeopenv1alpha1.PluginStatus, len(registry.Spec.Plugins))
	sem := make(chan struct{}, maxConcurrentChecks)

	var wg sync.WaitGroup
	for i, plugin := range registry.Spec.Plugins {
		wg.Add(1)
		go func(idx int, plugin kubeopenv1alpha1.RegistryPlugin) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			checkCtx, cancel := context.WithTimeout(ctx, statusCheckTimeout)
			defer cancel()

			now := metav1.Now()
			status := kubeopenv1alpha1.PluginStatus{
				Name:        plugin.Name,
				LastChecked: &now,
			}

			if strings.TrimSpace(plugin.Plugin.Name) == "" {
				status.Phase = kubeopenv1alpha1.AssetPhaseUnavailable
				status.Message = "plugin npm package name is empty"
				statuses[idx] = status
				return
			}

			// Check npm registry for package availability
			npmURL := registry.Spec.NpmRegistryURL
			version, err := r.checkNpmPackage(checkCtx, plugin.Plugin.Name, npmURL)
			if err != nil {
				status.Phase = kubeopenv1alpha1.AssetPhaseUnavailable
				status.Message = err.Error()
			} else {
				status.Phase = kubeopenv1alpha1.AssetPhaseReady
				status.ResolvedVersion = version
			}

			statuses[idx] = status
		}(i, plugin)
	}
	wg.Wait()

	return statuses
}

const defaultNpmRegistryURL = "https://registry.npmjs.org/"

// checkNpmPackage queries the npm registry API to validate a package exists
// and resolves its version. If registryURL is empty, the default npmjs.org is used.
func (r *RegistryReconciler) checkNpmPackage(ctx context.Context, packageSpec string, registryURL string) (string, error) {
	// Parse package name and version from npm specifier
	// Examples:
	//   "cc-safety-net"          -> name=cc-safety-net, version=latest
	//   "cc-safety-net@0.8.2"   -> name=cc-safety-net, version=0.8.2
	//   "@scope/pkg@1.0.0"      -> name=@scope/pkg, version=1.0.0
	name, version := parseNpmSpec(packageSpec)

	// Build registry URL
	baseURL := defaultNpmRegistryURL
	if registryURL != "" {
		baseURL = strings.TrimRight(registryURL, "/") + "/"
	}

	// URL-encode the package name (scoped packages contain '/')
	encodedName := strings.ReplaceAll(name, "/", "%2f")
	url := fmt.Sprintf("%s%s", baseURL, encodedName)

	httpClient := r.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: statusCheckTimeout}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create npm registry request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("npm registry request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("npm package not found: %s", name)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("npm registry returned status %d for package %s", resp.StatusCode, name)
	}

	// Read and parse the response to extract version info
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		return "", fmt.Errorf("failed to read npm registry response: %w", err)
	}

	var pkgInfo struct {
		DistTags map[string]string `json:"dist-tags"`
		Versions map[string]any    `json:"versions"`
	}
	if err := json.Unmarshal(body, &pkgInfo); err != nil {
		return "", fmt.Errorf("failed to parse npm registry response: %w", err)
	}

	// If a specific exact version was requested, check if it exists.
	// Semver range specifiers (^, ~, >=, etc.) cannot be matched by exact
	// lookup — for ranges, we fall back to returning the latest version
	// to confirm the package exists and is reachable.
	if version != "" && version != "latest" {
		if isSemverRange(version) {
			// Range specifier: package exists (HTTP 200), return latest as resolved version
			if latest, ok := pkgInfo.DistTags["latest"]; ok {
				return latest, nil
			}
		} else if _, ok := pkgInfo.Versions[version]; ok {
			// Exact version match
			return version, nil
		} else {
			return "", fmt.Errorf("npm package %s version %s not found", name, version)
		}
	}

	// Return the latest version
	if latest, ok := pkgInfo.DistTags["latest"]; ok {
		return latest, nil
	}

	return "", fmt.Errorf("npm package %s has no latest version", name)
}

// isSemverRange returns true if the version string is a semver range specifier
// rather than an exact version. Range specifiers start with ^, ~, >, <, =, or *.
func isSemverRange(version string) bool {
	if version == "" {
		return false
	}
	switch version[0] {
	case '^', '~', '>', '<', '=', '*':
		return true
	}
	// Also detect "x" ranges like "1.x" or "1.2.x"
	return strings.Contains(version, ".x") || strings.Contains(version, " ")
}

// parseNpmSpec splits an npm package specifier into name and version.
// Examples:
//
//	"pkg"          -> ("pkg", "")
//	"pkg@1.0.0"    -> ("pkg", "1.0.0")
//	"@scope/pkg"   -> ("@scope/pkg", "")
//	"@scope/pkg@1" -> ("@scope/pkg", "1")
func parseNpmSpec(spec string) (string, string) {
	// Handle scoped packages: @scope/pkg@version
	if strings.HasPrefix(spec, "@") {
		// Find the second '@' which separates name from version
		rest := spec[1:]
		if idx := strings.Index(rest, "@"); idx >= 0 {
			return spec[:idx+1], rest[idx+1:]
		}
		return spec, ""
	}

	// Non-scoped: pkg@version
	if idx := strings.Index(spec, "@"); idx >= 0 {
		return spec[:idx], spec[idx+1:]
	}
	return spec, ""
}

// SetupWithManager sets up the controller with the Manager.
func (r *RegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeopenv1alpha1.Registry{}).
		Complete(r)
}
