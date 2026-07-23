import { useEffect, useRef, useState, useCallback } from 'react';
import type { WebSocketMessage, LogChunkMessage } from '../types/ws-messages';

interface BuildLogsProps {
  projectId: string;
}

type ConnectionState =
  | 'connecting'
  | 'connected'
  | 'reconnecting'
  | 'connection_lost'
  | 'complete';

type BuildStatus = 'in_progress' | 'succeeded' | 'failed';

const MAX_RECONNECT_ATTEMPTS = 5;

function getWebSocketUrl(projectId: string, since?: string): string {
  const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:8080';
  const wsUrl = apiUrl.replace(/^http/, 'ws');
  const base = `${wsUrl}/api/v1/projects/${projectId}/logs`;
  return since ? `${base}?since=${encodeURIComponent(since)}` : base;
}

export function BuildLogs({ projectId }: BuildLogsProps) {
  const [logs, setLogs] = useState<LogChunkMessage[]>([]);
  const [connectionState, setConnectionState] = useState<ConnectionState>('connecting');
  const [buildStatus, setBuildStatus] = useState<BuildStatus>('in_progress');
  const [reconnectAttempt, setReconnectAttempt] = useState(0);

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastTimestampRef = useRef<string | undefined>(undefined);
  const logsEndRef = useRef<HTMLDivElement | null>(null);
  const attemptCountRef = useRef(0);
  const isCompleteRef = useRef(false);
  const connectRef = useRef<() => void>(() => {});

  const scrollToBottom = useCallback(() => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [logs, scrollToBottom]);

  const scheduleReconnect = useCallback(() => {
    attemptCountRef.current += 1;
    const attempt = attemptCountRef.current;

    if (attempt > MAX_RECONNECT_ATTEMPTS) {
      setConnectionState('connection_lost');
      return;
    }

    setReconnectAttempt(attempt);
    setConnectionState('reconnecting');

    const delay = Math.pow(2, attempt - 1) * 1000; // 1s, 2s, 4s, 8s, 16s
    reconnectTimeoutRef.current = setTimeout(() => {
      connectRef.current();
    }, delay);
  }, []);

  const connect = useCallback(() => {
    // Cleanup existing connection and timers
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    const url = getWebSocketUrl(projectId, lastTimestampRef.current);
    const ws = new WebSocket(url);
    wsRef.current = ws;

    if (attemptCountRef.current === 0) {
      setConnectionState('connecting');
    } else {
      setConnectionState('reconnecting');
    }

    ws.onopen = () => {
      setConnectionState('connected');
      attemptCountRef.current = 0;
      setReconnectAttempt(0);
    };

    ws.onmessage = (event: MessageEvent) => {
      const message: WebSocketMessage = JSON.parse(event.data as string);

      switch (message.type) {
        case 'log':
          setLogs((prev) => [...prev, message]);
          lastTimestampRef.current = message.timestamp;
          break;
        case 'complete':
          setBuildStatus(message.status);
          setConnectionState('complete');
          isCompleteRef.current = true;
          ws.close();
          break;
        case 'error':
          setLogs((prev) => [
            ...prev,
            { type: 'log', timestamp: new Date().toISOString(), message: `Error: ${message.message}` },
          ]);
          break;
      }
    };

    ws.onclose = (event) => {
      // Don't reconnect if build completed or clean close
      if (isCompleteRef.current) {
        return;
      }

      if (!event.wasClean) {
        scheduleReconnect();
      }
    };

    ws.onerror = () => {
      // onclose will fire after onerror, reconnect is handled there
    };
  }, [projectId, scheduleReconnect]);

  // Keep the ref in sync so scheduleReconnect can call the latest connect
  connectRef.current = connect;

  const handleManualRetry = useCallback(() => {
    attemptCountRef.current = 0;
    setReconnectAttempt(0);
    connect();
  }, [connect]);

  useEffect(() => {
    connect();
    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [connect]);

  return (
    <div className="flex flex-col h-full w-full">
      {/* Status Header */}
      <div className="flex items-center gap-2 px-4 py-2 border-b border-gray-200 bg-white">
        <StatusIndicator state={connectionState} buildStatus={buildStatus} />
        <span className="text-sm font-medium text-gray-700">
          {getStatusLabel(connectionState, buildStatus, reconnectAttempt)}
        </span>
      </div>

      {/* Connection Lost Banner */}
      {connectionState === 'connection_lost' && (
        <div className="flex items-center justify-between px-4 py-3 bg-red-50 border-b border-red-200">
          <span className="text-sm text-red-700">
            Connection lost. Unable to reconnect after {MAX_RECONNECT_ATTEMPTS} attempts.
          </span>
          <button
            onClick={handleManualRetry}
            className="px-3 py-1 text-sm font-medium text-white bg-red-600 rounded hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2"
          >
            Retry
          </button>
        </div>
      )}

      {/* Log Container */}
      <div className="flex-1 overflow-y-auto bg-gray-900 p-4 font-mono text-sm">
        {connectionState === 'connecting' && logs.length === 0 && (
          <div className="flex items-center gap-2 text-gray-400">
            <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-gray-400" />
            <span>Connecting to build logs...</span>
          </div>
        )}
        {logs.map((log, index) => (
          <div key={index} className="flex gap-3 leading-relaxed">
            <span className="text-gray-500 shrink-0 select-none">
              {formatTimestamp(log.timestamp)}
            </span>
            <span className="text-gray-100 whitespace-pre-wrap break-all">
              {log.message}
            </span>
          </div>
        ))}
        {connectionState === 'complete' && (
          <div
            className={`mt-4 px-3 py-2 rounded text-sm font-medium ${
              buildStatus === 'succeeded'
                ? 'bg-green-900/50 text-green-300'
                : 'bg-red-900/50 text-red-300'
            }`}
          >
            Build {buildStatus === 'succeeded' ? 'completed successfully' : 'failed'}.
          </div>
        )}
        <div ref={logsEndRef} />
      </div>
    </div>
  );
}

function StatusIndicator({
  state,
  buildStatus,
}: {
  state: ConnectionState;
  buildStatus: BuildStatus;
}) {
  switch (state) {
    case 'connecting':
    case 'reconnecting':
      return (
        <div className="animate-spin rounded-full h-3 w-3 border-b-2 border-indigo-600" />
      );
    case 'connected':
      return <div className="h-3 w-3 rounded-full bg-green-500" />;
    case 'connection_lost':
      return <div className="h-3 w-3 rounded-full bg-red-500" />;
    case 'complete':
      return (
        <div
          className={`h-3 w-3 rounded-full ${
            buildStatus === 'succeeded' ? 'bg-green-500' : 'bg-red-500'
          }`}
        />
      );
  }
}

function getStatusLabel(
  state: ConnectionState,
  buildStatus: BuildStatus,
  reconnectAttempt: number,
): string {
  switch (state) {
    case 'connecting':
      return 'Connecting...';
    case 'connected':
      return 'Build in progress';
    case 'reconnecting':
      return `Reconnecting (attempt ${reconnectAttempt}/${MAX_RECONNECT_ATTEMPTS})...`;
    case 'connection_lost':
      return 'Connection lost';
    case 'complete':
      return buildStatus === 'succeeded' ? 'Build succeeded' : 'Build failed';
  }
}

function formatTimestamp(timestamp: string): string {
  try {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', { hour12: false });
  } catch {
    return timestamp;
  }
}
