// Copyright Contributors to the KubeOpenCode project

package v1alpha1

// SkillSource defines a source of skills for an agent.
// Each SkillSource references a Git repository containing SKILL.md files
// organized as one-folder-per-skill.
//
// The controller clones the repository, optionally filters specific skill directories,
// and auto-injects the mount paths into OpenCode's skills.paths configuration.
// OpenCode then discovers and loads the skills automatically.
//
// Example:
//
//	skills:
//	- name: official-skills
//	  git:
//	    repository: https://github.com/anthropics/skills.git
//	    ref: main
//	    path: skills/
//	    names:
//	    - frontend-design
//	    - webapp-testing
//
// +kubebuilder:validation:XValidation:rule="has(self.git)",message="git source is required"
type SkillSource struct {
	// Name is a unique identifier for this skill source.
	// Used for logging, mount path generation (/skills/{name}), and deduplication.
	// Must be a valid DNS label (lowercase alphanumeric with hyphens).
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Git specifies a Git repository containing skills.
	// +optional
	Git *GitSkillSource `json:"git,omitempty"`
}

// GitSkillSource defines a Git repository as a skill source.
// The repository should contain SKILL.md files organized as one-folder-per-skill,
// following the standard skill format (Markdown with YAML frontmatter).
type GitSkillSource struct {
	// Repository is the Git repository URL.
	// Supported protocols: https://, http://, git@ (SSH).
	// Example: "https://github.com/anthropics/skills.git"
	// +required
	Repository string `json:"repository"`

	// Ref is the Git reference to checkout (branch, tag, or commit SHA).
	// Defaults to "HEAD" if not specified.
	// Example: "main", "v2.0.0", "abc123"
	// +optional
	// +kubebuilder:default="HEAD"
	Ref string `json:"ref,omitempty"`

	// Path is the base directory within the repository where skills are located.
	// For example, "skills/" for the anthropics/skills repo structure,
	// or "engineering/" for domain-organized repos.
	// If omitted, the repository root is used as the base directory.
	// +optional
	Path string `json:"path,omitempty"`

	// Names selects specific skill directories under Path.
	// Each name corresponds to a skill folder name containing SKILL.md.
	// If omitted, ALL skills found under Path are included.
	// Each name must be a simple directory name (no path separators or ".." allowed).
	//
	// Example: ["frontend-design", "webapp-testing"] selects only those two skills
	// from the repository, ignoring all others.
	// +optional
	// +listType=set
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=253
	// +kubebuilder:validation:items:Pattern=`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`
	Names []string `json:"names,omitempty"`

	// Depth specifies the clone depth for shallow cloning.
	// 1 means shallow clone (fastest, default), 0 means full clone.
	// +optional
	// +kubebuilder:default=1
	Depth *int `json:"depth,omitempty"`

	// RecurseSubmodules enables recursive cloning of Git submodules.
	// +optional
	RecurseSubmodules bool `json:"recurseSubmodules,omitempty"`

	// SecretRef references a Secret containing Git credentials.
	// The Secret should contain one of:
	//   - "username" + "password": For HTTPS token-based auth (password can be a PAT)
	//   - "ssh-privatekey": For SSH key-based auth
	//   - "app-id" + "app-installation-id" + "app-private-key": For GitHub App
	//     authentication; an installation access token is minted at runtime.
	// GitHub App credentials take precedence when present.
	// If not specified, anonymous clone is attempted.
	// Reuses the same Secret format as context Git.
	// +optional
	SecretRef *GitSecretReference `json:"secretRef,omitempty"`
}
