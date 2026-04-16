import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useParams } from 'react-router-dom';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { ClipboardAddon } from '@xterm/addon-clipboard';
import { WebglAddon } from '@xterm/addon-webgl';
import { WebLinksAddon } from '@xterm/addon-web-links';
import '@xterm/xterm/css/xterm.css';

interface ShareInfo {
  agentName: string;
  namespace: string;
  profile?: string;
}

type ConnectionState = 'connecting' | 'connected' | 'disconnected' | 'error';

function ShareTerminalPage() {
  const { token } = useParams<{ token: string }>();
  const [shareInfo, setShareInfo] = useState<ShareInfo | null>(null);
  const [connectionState, setConnectionState] = useState<ConnectionState>('connecting');
  const [errorMessage, setErrorMessage] = useState('');
  const [isMaximized, setIsMaximized] = useState(false);
  const termRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  const cleanup = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    if (terminalRef.current) {
      terminalRef.current.dispose();
      terminalRef.current = null;
    }
    fitAddonRef.current = null;
  }, []);

  // Fetch share info
  useEffect(() => {
    if (!token) return;

    fetch(`/s/${token}/info`)
      .then(async (res) => {
        if (!res.ok) {
          const data = await res.json().catch(() => ({ message: 'Invalid or expired share link' }));
          throw new Error(data.message || 'Invalid or expired share link');
        }
        return res.json();
      })
      .then((info: ShareInfo) => {
        setShareInfo(info);
      })
      .catch((err) => {
        setConnectionState('error');
        setErrorMessage(err.message);
      });
  }, [token]);

  // Initialize terminal once share info is loaded
  useEffect(() => {
    if (!shareInfo || !termRef.current || !token) return;

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: 'ui-monospace, "SF Mono", Menlo, Monaco, "Cascadia Mono", monospace',
      theme: {
        background: '#0c0a09',
        foreground: '#e7e5e4',
        cursor: '#10b981',
        selectionBackground: '#44403c',
      },
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.loadAddon(new ClipboardAddon());
    term.loadAddon(new WebLinksAddon());
    term.open(termRef.current);
    try { term.loadAddon(new WebglAddon()); } catch { /* fallback to canvas */ }
    fitAddon.fit();

    terminalRef.current = term;
    fitAddonRef.current = fitAddon;

    // Connect WebSocket
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${location.host}/s/${token}/terminal`;
    const ws = new WebSocket(wsUrl);
    ws.binaryType = 'arraybuffer';
    wsRef.current = ws;

    ws.onopen = () => {
      setConnectionState('connected');
      const dims = fitAddon.proposeDimensions();
      if (dims) {
        ws.send(JSON.stringify({ type: 'resize', cols: dims.cols, rows: dims.rows }));
      }
    };

    ws.onmessage = (event) => {
      if (event.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(event.data));
      }
    };

    ws.onclose = (event) => {
      setConnectionState('disconnected');
      if (event.code === 1011) {
        term.write('\r\n\x1b[90m[Session ended with error]\x1b[0m\r\n');
      } else {
        term.write('\r\n\x1b[90m[Session ended]\x1b[0m\r\n');
      }
    };

    ws.onerror = () => {
      setConnectionState('error');
      setErrorMessage('Connection error');
      term.write('\r\n\x1b[31m[Connection error]\x1b[0m\r\n');
    };

    // Send keystrokes
    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        const encoder = new TextEncoder();
        ws.send(encoder.encode(data));
      }
    });

    // Send resize events
    term.onResize(({ cols, rows }) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', cols, rows }));
      }
    });

    return cleanup;
  }, [shareInfo, token, cleanup]);

  // Re-fit on maximize/restore
  useEffect(() => {
    if (fitAddonRef.current) {
      const timer = setTimeout(() => fitAddonRef.current?.fit(), 200);
      return () => clearTimeout(timer);
    }
  }, [isMaximized]);

  // Handle window resize
  useEffect(() => {
    const handleResize = () => fitAddonRef.current?.fit();
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  // Error state
  if (connectionState === 'error' && !shareInfo) {
    return (
      <div className="min-h-screen bg-stone-950 flex items-center justify-center">
        <div className="text-center max-w-md px-6">
          <div className="w-14 h-14 rounded-2xl bg-red-500/10 border border-red-500/20 flex items-center justify-center mx-auto mb-5">
            <svg className="w-6 h-6 text-red-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="10" />
              <line x1="15" y1="9" x2="9" y2="15" />
              <line x1="9" y1="9" x2="15" y2="15" />
            </svg>
          </div>
          <h1 className="text-lg font-medium text-stone-200 mb-2">Share Link Unavailable</h1>
          <p className="text-sm text-stone-500">{errorMessage || 'This share link is invalid or has expired.'}</p>
        </div>
      </div>
    );
  }

  // Loading state
  if (!shareInfo) {
    return (
      <div className="min-h-screen bg-stone-950 flex items-center justify-center">
        <div className="text-center">
          <div className="w-5 h-5 border-2 border-stone-700 border-t-emerald-400 rounded-full animate-spin mx-auto mb-3" />
          <p className="text-sm text-stone-500">Connecting...</p>
        </div>
      </div>
    );
  }

  const statusDot = {
    connecting: 'bg-amber-400 animate-pulse',
    connected: 'bg-emerald-400',
    disconnected: 'bg-stone-600',
    error: 'bg-red-400',
  }[connectionState];

  return (
    <div className="min-h-screen bg-stone-950 flex flex-col">
      {/* Header */}
      <div className="px-4 py-2.5 bg-stone-900/80 flex items-center justify-between flex-shrink-0 border-b border-stone-800/60">
        <div className="flex items-center space-x-3">
          <div className="flex items-center space-x-2">
            <span className={`inline-block w-1.5 h-1.5 rounded-full ${statusDot}`} />
            <span className="text-[11px] font-medium text-stone-500 uppercase tracking-wider">Terminal</span>
          </div>
          <span className="text-sm text-stone-300 font-mono">{shareInfo.agentName}</span>
          <span className="text-[11px] text-stone-600">{shareInfo.namespace}</span>
        </div>
        <div className="flex items-center space-x-1">
          <button
            onClick={() => setIsMaximized(!isMaximized)}
            className="p-1.5 rounded-md text-stone-500 hover:text-stone-300 hover:bg-white/5 transition-colors"
            title={isMaximized ? 'Restore' : 'Maximize'}
          >
            {isMaximized ? (
              <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="4 14 10 14 10 20" />
                <polyline points="20 10 14 10 14 4" />
                <line x1="14" y1="10" x2="21" y2="3" />
                <line x1="3" y1="21" x2="10" y2="14" />
              </svg>
            ) : (
              <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="15 3 21 3 21 9" />
                <polyline points="9 21 3 21 3 15" />
                <line x1="21" y1="3" x2="14" y2="10" />
                <line x1="3" y1="21" x2="10" y2="14" />
              </svg>
            )}
          </button>
        </div>
      </div>

      {/* Terminal */}
      <div ref={termRef} className="flex-1 min-h-0" />

      {/* Footer */}
      <div className="px-4 py-1.5 bg-stone-900/50 border-t border-stone-800/40 flex items-center justify-between flex-shrink-0">
        <span className="text-[10px] text-stone-700">Powered by KubeOpenCode</span>
        {shareInfo.profile && (
          <span className="text-[10px] text-stone-600 truncate max-w-xs">{shareInfo.profile}</span>
        )}
      </div>
    </div>
  );
}

export default ShareTerminalPage;
