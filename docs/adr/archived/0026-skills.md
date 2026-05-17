# ADR 0026: Skills as a Top-Level Agent Field

## Status

Accepted

## Context

The AI agent ecosystem has standardized on SKILL.md as the format for defining reusable agent capabilities. Skills are organized as one-folder-per-skill in Git repositories (e.g., `anthropics/skills`). OpenCode already supports discovering skills from local directories via the `skills.paths` configuration in `opencode.json`.

KubeOpenCode needs to enable Agents to reference external skills from Git repositories, allowing teams to share and reuse skills across projects and organizations.

## Decision

We add `skills` as a **top-level field** on `AgentSpec` (and `AgentTemplateSpec`), rather than implementing it as a new context type.

### API

```yaml
spec:
  skills:
  - name: official-skills
    git:
      repository: https://github.com/anthropics/skills.git
      ref: main
      path: skills/
      names:
      - frontend-design
      - webapp-testing
      secretRef:
        name: github-creds
```

### Implementation

1. Controller clones skill Git repos via existing `git-init` init containers
2. Skills are mounted at `/skills/{source-name}/`
3. Controller auto-injects `skills.paths` into `opencode.json` configuration
4. OpenCode discovers and loads SKILL.md files automatically

## Alternatives Considered

### Skills as a Context subtype

Add a `Skills` value to the existing `ContextType` enum and handle skills within the context pipeline.

**Rejected because:**
- Contexts are knowledge ("what the agent knows"), skills are capabilities ("what the agent can do") - mixing them conflates semantics
- Skills require special handling (injecting `skills.paths` into OpenCode config) that doesn't fit the context processing pipeline
- Context mounting behavior (mount to path vs. aggregate to context.md) doesn't apply to skills
- Future skill-specific features (permissions, versioning) would be awkward as context extensions

### Skills as a separate CRD

Create a `Skill` CRD that Agents reference.

**Rejected because:**
- Premature abstraction - skills are a simple list of Git sources, not a complex lifecycle object
- Adds operational complexity (another resource to manage, RBAC to configure)
- Can be evolved to a CRD later if the use case grows

## Consequences

- Skills are first-class in the Agent API, making the intent clear
- Reuses existing `git-init` infrastructure, minimizing new code
- Template merge follows the same strategy as contexts (Agent replaces Template)
- `names` filter allows selective skill inclusion without cloning only parts of a repo
- No RBAC changes needed since skills don't introduce new Kubernetes resource types
