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

    // Close existing connection
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

  // Auto-scroll to bottom when new logs arrive (if autoScroll is on)
  useEffect(() => {
    if (autoScroll && logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
    }
  }, [logs, autoScroll]);

  // Detect manual scroll to disable auto-scroll
  const handleScroll = useCallback(() => {
    if (!logContainerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = logContainerRef.current;
    const isNearBottom = scrollHeight - scrollTop - clientHeight < 50;
    setAutoScroll(isNearBottom);
  }, []);

  // Toggle search with Ctrl+F
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

  // Filter logs by search query
  const filteredLogs = searchQuery
    ? logs.map((line, index) => ({ line, index, matches: line.toLowerCase().includes(searchQuery.toLowerCase()) }))
    : logs.map((line, index) => ({ line, index, matches: true }));

  const matchCount = searchQuery ? filteredLogs.filter((l) => l.matches).length : 0;

  const containerClass = isFullscreen
    ? 'fixed inset-0 z-40 bg-gray-900 flex flex-col'
    : 'bg-gray-900 rounded-lg overflow-hidden';

  const logAreaClass = isFullscreen
    ? 'flex-1 overflow-y-auto font-mono text-sm text-gray-100 whitespace-pre-wrap p-4'
    : 'p-4 h-96 overflow-y-auto font-mono text-sm text-gray-100 whitespace-pre-wrap';

  return (
    <div className={containerClass}>
      {/* Header */}
      <div className="px-4 py-2 bg-gray-800 flex items-center justify-between flex-shrink-0">
        <div className="flex items-center space-x-2">
          <span className="text-sm font-medium text-gray-300">Logs</span>
          <span
            className={`inline-block w-2 h-2 rounded-full ${
              isConnected ? 'bg-green-500' : 'bg-gray-500'
            }`}
          />
        </div>
        <div className="flex items-center space-x-3">
          <span className="text-xs text-gray-400">{status}</span>
          <button
            onClick={() => {
              setShowSearch(!showSearch);
              setTimeout(() => searchInputRef.current?.focus(), 0);
            }}
            className="text-gray-400 hover:text-gray-200"
            title="Search (Ctrl+F)"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          </button>
          <button
            onClick={handleDownload}
            className="text-gray-400 hover:text-gray-200"
            title="Download logs"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
          </button>
          <button
            onClick={() => setIsFullscreen(!isFullscreen)}
            className="text-gray-400 hover:text-gray-200"
            title={isFullscreen ? 'Exit fullscreen' : 'Fullscreen'}
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
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
        <div className="px-4 py-2 bg-gray-800 border-t border-gray-700 flex items-center space-x-2 flex-shrink-0">
          <input
            ref={searchInputRef}
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search logs..."
            className="flex-1 bg-gray-700 text-gray-100 text-sm rounded px-3 py-1 border border-gray-600 focus:outline-none focus:border-primary-500"
          />
          {searchQuery && (
            <span className="text-xs text-gray-400">{matchCount} matches</span>
          )}
          <button
            onClick={() => { setShowSearch(false); setSearchQuery(''); }}
            className="text-gray-400 hover:text-gray-200 text-xs"
          >
            Close
          </button>
        </div>
      )}

      {/* Error message */}
      {error && (
        <div className="px-4 py-2 bg-red-900/50 text-red-300 text-sm flex-shrink-0">{error}</div>
      )}

      {/* Log content */}
      <div
        ref={logContainerRef}
        onScroll={handleScroll}
        className={logAreaClass}
      >
        {logs.length === 0 ? (
          <span className="text-gray-500">
            {podName ? 'Waiting for logs...' : 'Pod not yet created'}
          </span>
        ) : (
          filteredLogs.map(({ line, index, matches }) => {
            if (searchQuery && !matches) return null;
            return (
              <div key={index} className="hover:bg-gray-800/50 flex">
                <span className="text-gray-600 select-none w-12 text-right pr-3 flex-shrink-0">
                  {index + 1}
                </span>
                <span className={searchQuery && matches ? 'bg-yellow-900/40' : ''}>
                  {line}
                </span>
              </div>
            );
          })
        )}
      </div>

      {/* Footer */}
      <div className="px-4 py-2 bg-gray-800 flex items-center justify-between flex-shrink-0">
        <span className="text-xs text-gray-500">{logs.length} lines</span>
        <div className="flex items-center space-x-3">
          {!autoScroll && (
            <button
              onClick={scrollToBottom}
              className="text-xs text-gray-400 hover:text-gray-200 flex items-center space-x-1"
            >
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 14l-7 7m0 0l-7-7m7 7V3" />
              </svg>
              <span>Scroll to bottom</span>
            </button>
          )}
          <button
            onClick={() => setLogs([])}
            className="text-xs text-gray-400 hover:text-gray-200"
          >
            Clear
          </button>
        </div>
      </div>
    </div>
  );
}

export default LogViewer;
