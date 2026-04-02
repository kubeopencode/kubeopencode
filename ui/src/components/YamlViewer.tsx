import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import CodeMirror from '@uiw/react-codemirror';
import { yaml as yamlLang } from '@codemirror/lang-yaml';
import { EditorView } from '@codemirror/view';
import { parse, stringify } from 'yaml';

interface YamlViewerProps {
  queryKey: string[];
  fetchYaml: () => Promise<string>;
  onSave?: (yaml: string) => Promise<void>;
}

interface YamlError {
  message: string;
  line?: number;
  col?: number;
}

function validateYaml(value: string): YamlError | null {
  if (!value.trim()) return null;
  try {
    parse(value);
    return null;
  } catch (err: unknown) {
    const e = err as { message?: string; linePos?: Array<{ line: number; col: number }> };
    const pos = e.linePos?.[0];
    return {
      message: e.message || 'Invalid YAML',
      line: pos?.line,
      col: pos?.col,
    };
  }
}

function formatYaml(value: string): { formatted: string; error: string | null } {
  try {
    const doc = parse(value);
    return { formatted: stringify(doc, { indent: 2, lineWidth: 0 }), error: null };
  } catch (err: unknown) {
    return { formatted: value, error: (err as Error).message || 'Failed to format' };
  }
}

const darkTheme = EditorView.theme({
  '&': {
    backgroundColor: '#1c1917',
    color: '#d6d3d1',
    fontSize: '12px',
  },
  '.cm-gutters': {
    backgroundColor: '#292524',
    color: '#78716c',
    border: 'none',
    borderRight: '1px solid #44403c',
  },
  '.cm-activeLineGutter': {
    backgroundColor: '#44403c',
  },
  '.cm-activeLine': {
    backgroundColor: '#29252480',
  },
  '.cm-cursor': {
    borderLeftColor: '#e7e5e4',
  },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': {
    backgroundColor: '#44403c !important',
  },
  '.cm-panels': {
    backgroundColor: '#1c1917',
    color: '#d6d3d1',
    borderBottom: '1px solid #292524',
  },
  '.cm-panels input': {
    backgroundColor: '#0c0a09',
    color: '#d6d3d1',
    border: '1px solid #44403c',
  },
  '.cm-panels button': {
    backgroundColor: '#292524',
    color: '#d6d3d1',
  },
  '.cm-searchMatch': {
    backgroundColor: '#854d0e80',
  },
  '.cm-searchMatch.cm-searchMatch-selected': {
    backgroundColor: '#a16207',
  },
});

function YamlViewer({ queryKey, fetchYaml, onSave }: YamlViewerProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editValue, setEditValue] = useState('');
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');
  const [copyLabel, setCopyLabel] = useState('Copy');

  const { data: yaml, isLoading, error, refetch } = useQuery({
    queryKey: [...queryKey, 'yaml'],
    queryFn: fetchYaml,
    enabled: isOpen,
  });

  useEffect(() => {
    if (yaml && editing) {
      setEditValue(yaml);
    }
  }, [yaml, editing]);

  const handleEdit = () => {
    setEditValue(yaml || '');
    setEditing(true);
    setSaveError('');
  };

  const handleCancel = () => {
    setEditing(false);
    setSaveError('');
  };

  const handleSave = async () => {
    if (!onSave) return;
    setSaving(true);
    setSaveError('');
    try {
      await onSave(editValue);
      setEditing(false);
      refetch();
    } catch (err) {
      setSaveError((err as Error).message || 'Failed to save');
    } finally {
      setSaving(false);
    }
  };

  const handleCopy = useCallback(() => {
    const text = editing ? editValue : yaml;
    if (text) {
      navigator.clipboard.writeText(text);
      setCopyLabel('Copied!');
      setTimeout(() => setCopyLabel('Copy'), 1500);
    }
  }, [editing, editValue, yaml]);

  const handleFormat = useCallback(() => {
    const { formatted, error: fmtErr } = formatYaml(editValue);
    if (!fmtErr) {
      setEditValue(formatted);
    }
    // If format fails, validation error will already show
  }, [editValue]);

  const yamlError = useMemo(() => {
    if (!editing) return null;
    return validateYaml(editValue);
  }, [editing, editValue]);

  const extensions = useMemo(() => [yamlLang(), darkTheme], []);
  const readOnlyExtensions = useMemo(
    () => [yamlLang(), darkTheme, EditorView.editable.of(false)],
    []
  );

  return (
    <div className="mt-6">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center space-x-2 text-xs font-display font-medium text-stone-400 hover:text-stone-600 uppercase tracking-wider transition-colors"
      >
        <svg
          className={`w-3.5 h-3.5 transform transition-transform ${isOpen ? 'rotate-90' : ''}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
        </svg>
        <span>YAML</span>
      </button>
      {isOpen && (
        <div className="mt-2 bg-stone-800 rounded-xl overflow-hidden border border-stone-700 animate-fade-in">
          <div className="px-4 py-2.5 bg-stone-700/50 flex items-center justify-between border-b border-stone-600/50">
            <span className="text-xs text-stone-400 font-display">Resource Definition</span>
            <div className="flex items-center gap-2">
              {(yaml || editing) && (
                <button
                  onClick={handleCopy}
                  className="text-[11px] text-stone-500 hover:text-stone-300 transition-colors font-medium"
                >
                  {copyLabel}
                </button>
              )}
              {editing && (
                <button
                  onClick={handleFormat}
                  className="text-[11px] text-amber-400 hover:text-amber-300 transition-colors font-medium"
                  title="Format YAML (fix indentation)"
                >
                  Format
                </button>
              )}
              {onSave && !editing && yaml && (
                <button
                  onClick={handleEdit}
                  className="text-[11px] text-sky-400 hover:text-sky-300 transition-colors font-medium"
                >
                  Edit
                </button>
              )}
            </div>
          </div>
          <div className="yaml-editor">
            {isLoading ? (
              <div className="p-4">
                <span className="text-stone-500 text-sm">Loading...</span>
              </div>
            ) : error ? (
              <div className="p-4">
                <span className="text-red-400 text-sm">Error: {(error as Error).message}</span>
              </div>
            ) : editing ? (
              <div>
                <CodeMirror
                  value={editValue}
                  onChange={setEditValue}
                  extensions={extensions}
                  theme="dark"
                  height="384px"
                  basicSetup={{
                    lineNumbers: true,
                    foldGutter: true,
                    highlightActiveLine: true,
                    bracketMatching: true,
                    searchKeymap: true,
                  }}
                />
                {yamlError && (
                  <div className="px-4 py-2 bg-red-950/50 border-t border-red-900/50 flex items-start gap-2">
                    <svg className="w-3.5 h-3.5 text-red-400 mt-0.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z" />
                    </svg>
                    <span className="text-xs text-red-400 font-mono">
                      {yamlError.line != null
                        ? `Line ${yamlError.line}, Col ${yamlError.col}: ${yamlError.message}`
                        : yamlError.message}
                    </span>
                  </div>
                )}
                {saveError && (
                  <div className="px-4 py-2">
                    <p className="text-xs text-red-400">{saveError}</p>
                  </div>
                )}
                <div className="flex items-center justify-end gap-2 px-4 py-3 border-t border-stone-700/50">
                  <button
                    onClick={handleCancel}
                    disabled={saving}
                    className="px-3 py-1.5 text-xs font-medium text-stone-400 bg-stone-800 rounded-lg hover:bg-stone-700 transition-colors disabled:opacity-50"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleSave}
                    disabled={saving || yamlError != null}
                    className="px-3 py-1.5 text-xs font-medium text-white bg-sky-600 rounded-lg hover:bg-sky-700 transition-colors disabled:opacity-50"
                    title={yamlError ? 'Fix YAML errors before saving' : undefined}
                  >
                    {saving ? 'Saving...' : 'Save'}
                  </button>
                </div>
              </div>
            ) : (
              <CodeMirror
                value={yaml || ''}
                extensions={readOnlyExtensions}
                theme="dark"
                height="auto"
                maxHeight="384px"
                basicSetup={{
                  lineNumbers: true,
                  foldGutter: true,
                  highlightActiveLine: false,
                  searchKeymap: true,
                }}
              />
            )}
          </div>
        </div>
      )}
    </div>
  );
}

export default YamlViewer;
