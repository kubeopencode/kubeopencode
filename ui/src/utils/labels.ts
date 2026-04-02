// Kubernetes label keys used for filtering
export const LABEL_AGENT = 'kubeopencode.io/agent';
export const LABEL_AGENT_TEMPLATE = 'kubeopencode.io/agent-template';
export const LABEL_CRONTASK = 'kubeopencode.io/crontask';

// Template filter sentinel values
export const FILTER_HAS_TEMPLATE = 'has-template';
export const FILTER_NO_TEMPLATE = 'no-template';

/**
 * Appends a label expression to an existing label selector string.
 * Handles empty base selectors correctly.
 */
export function appendLabelSelector(base: string, addition: string): string {
  return base ? `${base},${addition}` : addition;
}
