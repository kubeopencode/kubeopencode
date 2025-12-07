# Echo Agent Image for E2E Testing
# This is a simple test agent that echoes the content of task.md
FROM alpine:3.21

# Install minimal dependencies
RUN apk add --no-cache bash

# Build argument for workspace directory
ARG WORKSPACE_DIR=/workspace

# Set workspace directory as environment variable for runtime use
ENV WORKSPACE_DIR=${WORKSPACE_DIR}

# Create non-root user for security
RUN adduser -D -s /bin/bash agent
USER agent
WORKDIR ${WORKSPACE_DIR}

# Default entrypoint - simply echo the content of task.md
# This is useful for e2e testing to verify the task file is correctly mounted
ENTRYPOINT ["sh", "-c", "echo '=== Task Content ===' && cat ${WORKSPACE_DIR}/task.md && echo '' && echo '=== Task Completed ==='"]
