# code-server Sidecar Image
#
# This image provides a browser-based VSCode experience for humanInTheLoop debugging.
# It extends the universal base with code-server and Gemini CLI for remote development.
#
# Documentation:
# - code-server: https://github.com/coder/code-server
# - Gemini CLI: https://github.com/google-gemini/gemini-cli
#
# Usage in Agent configuration:
#
#   humanInTheLoop:
#     enabled: true
#     image: quay.io/kubetask/kubetask-agent-code-server:latest
#     command:
#       - sh
#       - -c
#       - code-server --bind-addr 0.0.0.0:8080 ${WORKSPACE_DIR} & sleep 7200
#     ports:
#       - name: code-server
#         containerPort: 8080
#
# Access via:
#   kubectl port-forward pod/<pod-name> 8080:8080
#   Then open: http://localhost:8080
#
# The Gemini CLI is available in the VSCode terminal for interactive use.
#
ARG BASE_IMAGE=quay.io/kubetask/kubetask-agent-base:latest
FROM ${BASE_IMAGE}

USER root

# Install Gemini CLI globally
RUN npm install -g @google/gemini-cli

# Install code-server
RUN curl -fsSL https://code-server.dev/install.sh | sh

USER agent

# Configure code-server: disable auth, bind to all interfaces
RUN mkdir -p ~/.config/code-server && \
    echo 'bind-addr: 0.0.0.0:8080' > ~/.config/code-server/config.yaml && \
    echo 'auth: none' >> ~/.config/code-server/config.yaml && \
    echo 'cert: false' >> ~/.config/code-server/config.yaml

# No ENTRYPOINT - command is specified by user in Agent.humanInTheLoop.command
