export interface LogChunkMessage {
  type: 'log';
  timestamp: string;
  message: string;
}

export interface LogCompletionMessage {
  type: 'complete';
  status: 'succeeded' | 'failed';
}

export interface LogErrorMessage {
  type: 'error';
  message: string;
}

export type WebSocketMessage = LogChunkMessage | LogCompletionMessage | LogErrorMessage;
