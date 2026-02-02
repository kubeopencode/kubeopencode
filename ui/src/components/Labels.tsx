interface LabelsProps {
  labels?: Record<string, string>;
  maxDisplay?: number;
}

function Labels({ labels, maxDisplay }: LabelsProps) {
  if (!labels || Object.keys(labels).length === 0) return null;

  const entries = Object.entries(labels);
  const displayEntries = maxDisplay ? entries.slice(0, maxDisplay) : entries;
  const remaining = maxDisplay ? Math.max(0, entries.length - maxDisplay) : 0;

  return (
    <div className="flex flex-wrap gap-1">
      {displayEntries.map(([key, value]) => (
        <span
          key={key}
          className="inline-flex items-center px-2 py-0.5 rounded text-xs bg-gray-100 text-gray-700"
          title={`${key}=${value}`}
        >
          <span className="text-gray-500">{key}=</span>
          <span>{value}</span>
        </span>
      ))}
      {remaining > 0 && (
        <span className="text-xs text-gray-500">+{remaining} more</span>
      )}
    </div>
  );
}

export default Labels;
