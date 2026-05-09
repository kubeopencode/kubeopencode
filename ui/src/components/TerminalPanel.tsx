import React, { useState, useEffect, useCallback, useRef } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { ClipboardAddon } from '@xterm/addon-clipboard';
import { WebglAddon } from '@xterm/addon-webgl';
import { WebLinksAddon } from '@xterm/addon-web-links';
import '@xterm/xterm/css/xterm.css';

interface TerminalPanelProps {
  namespace: string;
  agentName: string;
  defaultMode?: PanelMode;
}

type PanelMode = 'collapsed' | 'expanded' | 'maximized';

function TerminalPanel({ namespace, agentName, defaultMode = 'collapsed' }: TerminalPanelProps) {
  const [mode, setMode] = useState<PanelMode>(defaultMode);
  const termRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [connected, setConnected] = useState(false);

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
    setConnected(false);
  }, []);

  // Initialize terminal when expanded/maximized
  useEffect(() => {
    if (mode === 'collapsed' || !termRef.current) {
      return;
    }

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: 'ui-monospace, "SF Mono", Menlo, Monaco, "Cascadia Mono", monospace',
      theme: {
        background: '#0c0a09', // stone-950
        foreground: '#e7e5e4', // stone-200
        cursor: '#10b981',     // emerald-500
        selectionBackground: '#44403c', // stone-700
      },
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.loadAddon(new ClipboardAddon());
    term.loadAddon(new WebLinksAddon());
    term.open(termRef.current);
    try { term.loadAddon(new WebglAddon()); } catch { /* fallback to canvas renderer */ }
    fitAddon.fit();

    terminalRef.current = term;
    fitAddonRef.current = fitAddon;

    // Connect WebSocket
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${location.host}/api/v1/namespaces/${namespace}/agents/${agentName}/terminal`;
    const ws = new WebSocket(wsUrl);
    ws.binaryType = 'arraybuffer';
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      // Send initial terminal size
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
      setConnected(false);
      if (event.code === 1011) {
        // Error close - the error message was already written to the terminal as binary data
        term.write('\r\n\x1b[90m[Session ended with error]\x1b[0m\r\n');
      } else {
        term.write('\r\n\x1b[90m[Session ended]\x1b[0m\r\n');
      }
    };

    ws.onerror = () => {
      setConnected(false);
      term.write('\r\n\x1b[31m[Connection error]\x1b[0m\r\n');
    };

    // Send keystrokes to server
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
  }, [mode, namespace, agentName, cleanup]);

  // Re-fit terminal when mode changes between expanded/maximized
  useEffect(() => {
    if (mode !== 'collapsed' && fitAddonRef.current) {
      // Small delay to allow CSS transitions to complete
      const timer = setTimeout(() => fitAddonRef.current?.fit(), 200);
      return () => clearTimeout(timer);
    }
  }, [mode]);

  // Handle window resize
  useEffect(() => {
    if (mode === 'collapsed') return;
    const handleResize = () => fitAddonRef.current?.fit();
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [mode]);

  // ESC key to exit maximized mode
  useEffect(() => {
    if (mode !== 'maximized') return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && e.ctrlKey) {
        e.stopPropagation();
        setMode('expanded');
      }
    };
    document.addEventListener('keydown', handler, true);
    return () => document.removeEventListener('keydown', handler, true);
  }, [mode]);

  // Collapsed: launch CTA card
  if (mode === 'collapsed') {
    return (
      <div>
        <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-3">Terminal</h3>
        <div
          className="group relative overflow-hidden rounded-xl border border-stone-200 bg-gradient-to-br from-stone-900 via-stone-900 to-stone-800 cursor-pointer transition-all duration-200 hover:border-emerald-600/40 hover:shadow-lg hover:shadow-emerald-900/10"
          onClick={() => setMode('expanded')}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => e.key === 'Enter' && setMode('expanded')}
        >
          <div className="absolute inset-0 opacity-[0.03]" style={{
            backgroundImage: 'linear-gradient(rgba(255,255,255,.1) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,.1) 1px, transparent 1px)',
            backgroundSize: '20px 20px'
          }} />
          <div className="relative px-5 py-4 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-9 h-9 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center group-hover:bg-emerald-500/20 transition-colors">
                <svg className="w-4.5 h-4.5 text-emerald-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <polyline points="4 17 10 11 4 5" />
                  <line x1="12" y1="19" x2="20" y2="19" />
                </svg>
              </div>
              <div>
                <span className="text-sm font-medium text-stone-200 group-hover:text-white transition-colors">
                  Launch Terminal
                </span>
                <p className="text-[11px] text-stone-500 mt-0.5">Interactive OpenCode TUI session in your browser</p>
              </div>
            </div>
            <svg className="w-4 h-4 text-stone-600 group-hover:text-emerald-400 group-hover:translate-x-0.5 transition-all" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M5 12h14M12 5l7 7-7 7" />
            </svg>
          </div>
        </div>
      </div>
    );
  }

  // Expanded / Maximized
  const isMaximized = mode === 'maximized';

  return (
    <>
      {isMaximized && (
        <div className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm" style={{ animation: 'fade-in 0.15s ease-out' }} />
      )}

      <div
        className={
          isMaximized
            ? 'fixed inset-3 z-50 bg-stone-950 flex flex-col rounded-2xl overflow-hidden shadow-2xl ring-1 ring-white/10'
            : 'bg-stone-950 rounded-xl overflow-hidden border border-stone-800 animate-fade-in flex flex-col'
        }
        style={isMaximized ? { animation: 'panel-maximize 0.2s cubic-bezier(0.16, 1, 0.3, 1)' } : undefined}
      >
        {/* Header bar */}
        <div className="px-4 py-2 bg-stone-900/80 flex items-center justify-between flex-shrink-0 border-b border-stone-800/60">
          <div className="flex items-center space-x-2.5">
            <span className={`inline-block w-1.5 h-1.5 rounded-full ${connected ? 'bg-emerald-400' : 'bg-stone-600 animate-pulse'}`} />
            <span className="text-[11px] font-display font-medium text-stone-500 uppercase tracking-wider">Terminal</span>
            <span className="text-[11px] text-stone-600 font-mono">{agentName}</span>
          </div>
          <div className="flex items-center">
            <button
              onClick={() => setMode(isMaximized ? 'expanded' : 'maximized')}
              className="p-1.5 rounded-md text-stone-500 hover:text-stone-300 hover:bg-white/5 transition-colors"
              title={isMaximized ? 'Restore (Ctrl+Esc)' : 'Maximize'}
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
            <div className="w-px h-4 bg-stone-700/50 mx-1.5" />
            <button
              onClick={() => { cleanup(); setMode('collapsed'); }}
              className="p-1.5 rounded-md text-stone-500 hover:text-red-400 hover:bg-red-500/10 transition-colors"
              title="Close"
            >
              <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          </div>
        </div>

        {/* Terminal container */}
        <div
          ref={termRef}
          className={isMaximized ? 'flex-1 min-h-0' : ''}
          style={{ height: isMaximized ? '100%' : 'calc(100vh - 280px)', minHeight: '400px' }}
        />
      </div>
    </>
  );
}

export default TerminalPanel;
