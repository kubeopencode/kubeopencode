# Claude Code Agent Image
#
# This image extends the universal base with Claude Code CLI for AI-powered tasks.
#
# Documentation:
# - Claude Code: https://code.claude.com/docs
# - Headless Mode: https://code.claude.com/docs/en/headless
#
# Environment Variables (set at runtime via KubeTask Agent secrets):
# - ANTHROPIC_API_KEY: Anthropic API key
#
ARG BASE_IMAGE=quay.io/kubetask/kubetask-agent-base:latest
FROM ${BASE_IMAGE}

# Install Claude Code CLI using the official install script
# The script installs to ~/.local/bin/claude
USER agent
RUN curl -fsSL https://claude.ai/install.sh | bash
ENV PATH="/home/agent/.local/bin:${PATH}"

# Build argument for workspace directory (inherited from base)
ARG WORKSPACE_DIR=/workspace
ENV WORKSPACE_DIR=${WORKSPACE_DIR}

# Default entrypoint - run Claude Code in headless mode
# The agent reads the task from ${WORKSPACE_DIR}/task.md and executes it
# Using -p flag for direct prompt input from the mounted context file
# Using --dangerously-skip-permissions to auto-approve actions (safe in container environment)
# Using stream-json output to see all logs
ENTRYPOINT ["sh", "-c", "claude --output-format stream-json --dangerously-skip-permissions -p \"$(cat ${WORKSPACE_DIR}/task.md)\""]
