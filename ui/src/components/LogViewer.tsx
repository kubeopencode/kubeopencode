import React, { useEffect, useRef, useState, useCallback } from 'react';
import api, { LogEvent } from '../api/client';

interface LogViewerProps {
  namespace: string;
  taskName: string;
  podName?: string;
  isRunning: boolean;
}

function LogViewer({ namespace, taskName, podName, isRunning }: LogViewerProps) {
  const [logs, setLogs] = useState<string[]>([]);
  const [status, setStatus] = useState<string>('Connecting...');
  const [error, setError] = useState<string | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [showSearch, setShowSearch] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const logContainerRef = useRef<HTMLDivElement>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!podName) {
      setStatus('Waiting for pod...');
      return;
    }

    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    const url = api.getTaskLogsUrl(namespace, taskName);
    const eventSource = new EventSource(url);
    eventSourceRef.current = eventSource;

    eventSource.onopen = () => {
      setIsConnected(true);
      setError(null);
      setStatus('Connected');
    };

    eventSource.onmessage = (event) => {
      try {
        const data: LogEvent = JSON.parse(event.data);

        switch (data.type) {
          case 'status':
            setStatus(`Pod: ${data.podPhase || data.phase}`);
            break;
          case 'log':
            if (data.content) {
              setLogs((prev) => [...prev, data.content!]);
            }
            break;
          case 'info':
            setStatus(data.message || 'Initializing...');
            break;
          case 'error':
            setError(data.message || 'Unknown error');
            break;
          case 'complete':
            setStatus(`Completed (${data.phase})`);
            setIsConnected(false);
            eventSource.close();
            break;
        }
      } catch (e) {
        console.error('Failed to parse log event:', e);
      }
    };

    eventSource.onerror = () => {
      setIsConnected(false);
      if (isRunning) {
        setStatus('Connection lost, reconnecting...');
      } else {
        setStatus('Stream ended');
        eventSource.close();
      }
    };

    return () => {
      eventSource.close();
    };
  }, [namespace, taskName, podName, isRunning]);

  useEffect(() => {
    if (autoScroll && logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
    }
  }, [logs, autoScroll]);

  const handleScroll = useCallback(() => {
    if (!logContainerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = logContainerRef.current;
    const isNearBottom = scrollHeight - scrollTop - clientHeight < 50;
    setAutoScroll(isNearBottom);
  }, []);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
        e.preventDefault();
        setShowSearch((prev) => !prev);
        setTimeout(() => searchInputRef.current?.focus(), 0);
      }
      if (e.key === 'Escape' && showSearch) {
        setShowSearch(false);
        setSearchQuery('');
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [showSearch]);

  const scrollToBottom = () => {
    if (logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
      setAutoScroll(true);
    }
  };

  const handleDownload = () => {
    const content = logs.join('');
    const blob = new Blob([content], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${taskName}-logs.txt`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const filteredLogs = searchQuery
    ? logs.map((line, index) => ({ line, index, matches: line.toLowerCase().includes(searchQuery.toLowerCase()) }))
    : logs.map((line, index) => ({ line, index, matches: true }));

  const matchCount = searchQuery ? filteredLogs.filter((l) => l.matches).length : 0;

  const containerClass = isFullscreen
    ? 'fixed inset-0 z-40 bg-stone-950 flex flex-col'
    : 'bg-stone-950 rounded-xl overflow-hidden border border-stone-800';

  const logAreaClass = isFullscreen
    ? 'flex-1 overflow-y-auto font-mono text-xs text-stone-300 whitespace-pre-wrap p-4 sidebar-scroll'
    : 'p-4 h-96 overflow-y-auto font-mono text-xs text-stone-300 whitespace-pre-wrap sidebar-scroll';

  return (
    <div className={containerClass}>
      {/* Header */}
      <div className="px-4 py-2.5 bg-stone-900 flex items-center justify-between flex-shrink-0 border-b border-stone-800">
        <div className="flex items-center space-x-2.5">
          <span className="text-xs font-display font-medium text-stone-400 uppercase tracking-wider">Logs</span>
          <span
            className={`inline-block w-1.5 h-1.5 rounded-full ${
              isConnected ? 'bg-emerald-400' : 'bg-stone-600'
            }`}
          />
        </div>
        <div className="flex items-center space-x-3">
          <span className="text-[11px] text-stone-500 font-mono">{status}</span>
          <button
            onClick={() => {
              setShowSearch(!showSearch);
              setTimeout(() => searchInputRef.current?.focus(), 0);
            }}
            className="text-stone-500 hover:text-stone-300 transition-colors"
            title="Search (Ctrl+F)"
          >
            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          </button>
          <button
            onClick={handleDownload}
            className="text-stone-500 hover:text-stone-300 transition-colors"
            title="Download logs"
          >
            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
          </button>
          <button
            onClick={() => setIsFullscreen(!isFullscreen)}
            className="text-stone-500 hover:text-stone-300 transition-colors"
            title={isFullscreen ? 'Exit fullscreen' : 'Fullscreen'}
          >
            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              {isFullscreen ? (
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              ) : (
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
              )}
            </svg>
          </button>
        </div>
      </div>

      {/* Search bar */}
      {showSearch && (
        <div className="px-4 py-2 bg-stone-900 border-b border-stone-800 flex items-center space-x-2 flex-shrink-0">
          <input
            ref={searchInputRef}
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search logs..."
            className="flex-1 bg-stone-800 text-stone-200 text-xs rounded-lg px-3 py-1.5 border border-stone-700 focus:outline-none focus:border-primary-500 placeholder:text-stone-600"
          />
          {searchQuery && (
            <span className="text-[11px] text-stone-500 font-mono">{matchCount} matches</span>
          )}
          <button
            onClick={() => { setShowSearch(false); setSearchQuery(''); }}
            className="text-stone-500 hover:text-stone-300 text-xs transition-colors"
          >
            Close
          </button>
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="px-4 py-2 bg-red-950/50 text-red-400 text-xs flex-shrink-0 border-b border-red-900/30">{error}</div>
      )}

      {/* Log content */}
      <div
        ref={logContainerRef}
        onScroll={handleScroll}
        className={logAreaClass}
      >
        {logs.length === 0 ? (
          <span className="text-stone-600">
            {podName ? 'Waiting for logs...' : 'Pod not yet created'}
          </span>
        ) : (
          filteredLogs.map(({ line, index, matches }) => {
            if (searchQuery && !matches) return null;
            return (
              <div key={index} className="hover:bg-stone-900/50 flex leading-5">
                <span className="text-stone-700 select-none w-10 text-right pr-3 flex-shrink-0">
                  {index + 1}
                </span>
                <span className={searchQuery && matches ? 'bg-amber-900/30' : ''}>
                  {line}
                </span>
              </div>
            );
          })
        )}
      </div>

      {/* Footer */}
      <div className="px-4 py-2 bg-stone-900 flex items-center justify-between flex-shrink-0 border-t border-stone-800">
        <span className="text-[11px] text-stone-600 font-mono">{logs.length} lines</span>
        <div className="flex items-center space-x-3">
          {!autoScroll && (
            <button
              onClick={scrollToBottom}
              className="text-[11px] text-stone-500 hover:text-stone-300 flex items-center space-x-1 transition-colors"
            >
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 14l-7 7m0 0l-7-7m7 7V3" />
              </svg>
              <span>Bottom</span>
            </button>
          )}
          <button
            onClick={() => setLogs([])}
            className="text-[11px] text-stone-500 hover:text-stone-300 transition-colors"
          >
            Clear
          </button>
        </div>
      </div>
    </div>
  );
}

export default LogViewer;
