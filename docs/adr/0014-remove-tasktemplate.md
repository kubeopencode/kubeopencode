# 14. Remove TaskTemplate CRD

Date: 2026-03-10

## Status

Accepted

## Context

KubeOpenCode had three layers of API abstraction: **Task** (what to do) -> **TaskTemplate** (reusable template) -> **Agent** (how to execute). The design philosophy evolved toward "Task should be simple, Agent should be complex", making TaskTemplate redundant as a middle layer.

Key observations:
- TaskTemplate's capabilities (contexts, agentRef, description) can all live directly in Agent
- Agent already supports a `contexts` field that can carry TaskTemplate's context
- With the skill progressive-loading mechanism, Agent can have extensive context without overwhelming the context window
- The three-layer context merge priority (Agent -> TaskTemplate -> Task) was confusing for users
- Multiple configurations can be handled by creating multiple Agents or using Agent skills/sub-agents

## Decision

Remove the TaskTemplate CRD entirely, simplifying the API to a pure **Task + Agent** dual-API model:

- **Task** = WHAT (the task to execute, with optional inline contexts)
- **Agent** = HOW (the execution configuration, with contexts, credentials, skills)

All TaskTemplate functionality is absorbed by Agent:
- Template `contexts` -> Agent `contexts`
- Template `agentRef` -> Task directly specifies `agentRef`
- Template `description` -> Task directly specifies `description`

## Consequences

### Positive
- API simplified from 3 core CRDs to 2 (Task + Agent)
- Clearer responsibility separation: Task = WHAT, Agent = HOW
- Reduced maintenance burden (~500 lines of controller merge logic, ~300 lines of handlers removed)
- Eliminated confusing three-layer context merge priority
- Agent builders can encapsulate all configuration internally
- GitOps-friendly: users manage only Task and Agent YAML

### Negative
- Breaking change for users of TaskTemplate (acceptable in v1alpha1)
- Loss of "shared template" semantics (mitigated by creating multiple Agents or using Helm values)
- Tasks may need more explicit configuration without template defaults

### Migration Path
- Users referencing TaskTemplates should move template contexts to their Agent
- Users using template agentRef should specify agentRef directly in Task
- Users using template description should specify description directly in Task
