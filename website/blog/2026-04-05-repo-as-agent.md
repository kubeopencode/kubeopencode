---
slug: repo-as-agent
title: "Whatever You Want to Build, Build an Agent First"
authors: [kubeopencode]
tags: [architecture, agents, patterns]
---

Whatever you want to build, build an agent first — then let the agent build it. Here's how to do it with nothing more than a Git repo.

{/* truncate */}

## The Problem

If you work across multiple repositories — upstream libraries, downstream forks, multiple release branches — you already know the pain. Context-switching between repos, tracing cross-repo dependencies, and running through repetitive CI/PR workflows consumes a significant chunk of engineering time.

Our answer: before building anything else, build an agent first. Then use that agent to do the actual work.

## The Repo-as-Agent Pattern

The core idea is a shift in mindset: whatever you want to build, don't start by writing product code — start by building an agent, then let the agent build the product.

In practice, this means creating a Git repository that is **not** your product code. It **is** the agent itself.

## How We Built This Agent

Once you have the repo, the next question is: what do you put in it? Here's what we found matters most.

### 1. Agent Identity, Not Business Documentation

If you've used AI coding agents before, you're probably familiar with `CLAUDE.md` (or similar files like `AGENTS.md`, `COPILOT.md`). In a typical product repo, these files describe that specific codebase — how to build it, how to test it, what conventions to follow. They are scoped to one business component.

In a Repo-as-Agent setup, this changes fundamentally. The `CLAUDE.md` and `README` in the agent repo don't describe any particular business component. They define the **agent's identity** — its behavioral rules, its operating principles, and how it should interact with the outside world. This is decoupled from any specific product code.

When the agent actually needs to work on a specific product repo — developing a feature, fixing a bug — it reads that repo's own `CLAUDE.md` for business context at that point. But the agent repo's `CLAUDE.md` is a layer above: it governs how the agent **behaves as an agent**, not how any one service works.

### 2. Cross-Repo Visibility

This is where the agent gets its full view of the world. We bring all related repositories into a single `repos/` directory as read-only reference copies. The agent doesn't work inside these repos; it reads across all of them, just like a human engineer who has the entire codebase in their head — but at full scope, all the time.

When the agent needs to actually modify code, it creates isolated Git worktree checkouts in a separate directory — one per task, no interference with the read-only view.

But putting repos side by side is not enough. You also need to explicitly document the relationships between them — which repo depends on which, at what layer, and how changes propagate. We maintain a repo registry and dependency documentation that maps out the full dependency chain. Without this, the agent is just searching blindly across directories. With it, the agent can trace a bug through the correct dependency path, understand which repos are affected by a version bump, and reason about cross-repo impact — just like a senior engineer would.

### 3. Tool Integration

Just like you give a human engineer access to tools, you need to give the agent tools. We connected three basic ones:

- **GitHub** — PR status, code review, and PR creation
- **Issue tracking** — issue search, creation, status updates
- **Slack** — formatted notifications to team channels

Even with just these basics, the agent handles a surprising amount of real work.

### 4. Team Knowledge

This is something easily overlooked. When you configure system prompts for individual repos, you typically focus on code — build instructions, test commands, coding conventions. But if you're building an agent that will integrate deeper into your workflow over time, it needs to understand more than code. It needs to know your team.

We maintain information about each engineer — their name, GitHub username, email, and component ownership. This lets the agent know who to assign issues to, who to notify about a failing PR, and which engineer owns which part of the system. As the agent takes on more responsibilities, this organizational context becomes increasingly important.

### 5. Scheduled Workflows

An agent that only works when you talk to it is useful, but an agent that works on a schedule — without being asked — is where things get interesting. We use [KubeOpenCode](https://kubeopencode.github.io/kubeopencode/), a Kubernetes-native platform for running AI agents, to deploy our agent with [CronTask](/docs/features/crontask)-based scheduled workflows.

Here are some examples:

- **Daily PR Review** — Every day, the agent reviews all open PRs that haven't been AI-reviewed yet, posts detailed code review comments, and flags issues.
- **Weekly Vulnerability Fix** — Every week, the agent checks for open Dependabot security alerts, analyzes them, and creates fix PRs for vulnerable dependencies.
- **Periodic Refactoring** — The agent identifies small refactoring opportunities and submits incremental cleanup PRs.

The key: the agent doesn't just forward information. Before reporting, it has already analyzed the relevant code and taken action where possible.

### 6. Event-Driven Responses

Beyond scheduled workflows, the agent also responds to real-time events:

- **GitHub @mentions** — When mentioned in a PR comment or issue, the agent reads the context, answers questions, makes code changes, or creates follow-up PRs.
- **Slack messages** — When messaged in a team channel, the agent can look up information, run analyses, and respond with findings.

This turns the agent from a batch processor into a team member that's always available.

## Deploying the Agent

We deploy the agent on Kubernetes using KubeOpenCode. Our agent repo — [kubeopencode-agent](https://github.com/kubeopencode/kubeopencode-agent) — is a living example of this pattern. It contains the agent's identity, cross-repo context, team knowledge, skills, and workflow definitions. The deployment includes:

- An [`AgentTemplate`](/docs/features/agent-templates) that defines the agent's base configuration — which AI model to use via the [`config` field](/docs/setting-up-agent#step-1-configure-the-ai-model), and how repos are synced via [Git contexts](/docs/setting-up-agent#context)
- [`Skills`](/docs/setting-up-agent#skills) imported from Git repositories — reusable AI capabilities defined as `SKILL.md` files
- [`CronTask`](/docs/features/crontask) resources that schedule each workflow independently
- [`Credentials`](/docs/setting-up-agent#providing-api-keys) for GitHub tokens and API keys, managed as Kubernetes Secrets

The entire deployment is defined as Kubernetes manifests, version-controlled alongside the agent code. Infrastructure as code, but for your AI agent. See [Setting Up an Agent](/docs/setting-up-agent) for a step-by-step guide.

## Known Gaps

This is still early. Here are the gaps we've identified so far.

### Access and Permissions

Giving an agent tools is one thing. Getting the permissions for those tools to actually work in a real environment is another — and it turns out to be harder than expected. The agent frequently gets stuck at execution time because it can't reach certain test environments or can't access internal services that a human engineer would reach through their local setup.

Slack is a good example. We can send one-way notifications via webhook, but setting up a real-time Slack bot that lets the agent interact with engineers conversationally requires additional infrastructure and permissions. Without it, many scenarios still require a human to copy-paste information between the agent and communication channels, which caps the level of automation we can achieve.

### Long-Term Memory

Each session starts fresh. The agent has no record of what happened yesterday, last week, or last sprint. This matters more than you might think.

For example, if your team is in the middle of a migration and a new bug comes in related to the component being migrated, a human engineer immediately suspects a connection — that context is in their head. But the agent doesn't know the migration is happening. It analyzes the bug in isolation, missing the most likely root cause.

Engineering activity — what's being migrated, what decisions were made, what broke recently and why — is critical context for understanding new problems. How to capture daily engineering activity and decisions as long-term memory that the agent can access is an open problem we're actively thinking about.

## The Future

One thing we want to be clear about: the agent engine — Claude Code, Codex, or whatever comes next — is not what matters most. These are runtimes. They will evolve, and they are swappable.

What matters is what you build on top: knowledge management, permissions, workflows, business context, and organizational information. Once you've built that foundation, you can migrate it across runtimes, or even compose multiple agent frameworks together for different purposes.

We believe the future looks like this: every team, in every specific business domain, will have a highly customized agent built for their own needs. There may be shared layers — common libraries, common tool integrations, common infrastructure — but each team's agent will be unique, deeply tailored to how that team actually works.

This is a basic version, and we know it. But it's a good start.
