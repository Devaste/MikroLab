// MikroLab WebSocket API Client
// Sends versioned CommandRequest messages and receives CommandResponse/EventMessage

import Ajv from 'ajv';
import type { CommandRequest, CommandResponse } from './schemas/index.d';

const API_VERSION = '1.0';
const WS_URL = `ws://${window.location.hostname}:8080/ws`;

/**
 * Pending request stored while waiting for a matching response.
 */
interface PendingRequest {
  resolve: (value: CommandResponse) => void;
  reject: (reason: unknown) => void;
  timer: ReturnType<typeof setTimeout>;
}

export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'error';

/**
 * WebSocket client for the MikroLab API with auto-reconnect support.
 *
 * Usage:
 *   const client = new ApiClient();
 *   client.onStatusChange((status) => console.log(status));
 *   await client.connect();
 *   const resp = await client.sendCommand('/ip/address/print');
 *   console.log(resp.result);
 */
export class ApiClient {
  private ws: WebSocket | null = null;
  private pending = new Map<number, PendingRequest>();
  private nextId = 1;
  private ajv: Ajv;
  private validateRequest: ReturnType<Ajv['compile']> | null = null;
  private eventHandlers: Map<string, Set<(payload: unknown) => void>> = new Map();
  private status: ConnectionStatus = 'disconnected';
  private statusHandlers: Set<(status: ConnectionStatus) => void> = new Set();
  private url: string = WS_URL;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private shouldReconnect = false;

  constructor(autoReconnect = true) {
    this.ajv = new Ajv();
    this.shouldReconnect = autoReconnect;
  }

  /**
   * Register a handler for connection status changes.
   */
  onStatusChange(handler: (status: ConnectionStatus) => void): () => void {
    this.statusHandlers.add(handler);
    return () => {
      this.statusHandlers.delete(handler);
    };
  }

  /**
   * Get the current connection status.
   */
  getStatus(): ConnectionStatus {
    return this.status;
  }

  private setStatus(newStatus: ConnectionStatus): void {
    this.status = newStatus;
    for (const handler of this.statusHandlers) {
      try {
        handler(newStatus);
      } catch (err) {
        console.error('Status handler error:', err);
      }
    }
  }

  private scheduleReconnect(): void {
    if (!this.shouldReconnect) return;
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      this.setStatus('error');
      return;
    }
    this.reconnectAttempts++;
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts - 1), 30000);
    this.reconnectTimer = setTimeout(() => {
      this.internalConnect();
    }, delay);
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  /**
   * Connect to the WebSocket server and perform the version handshake.
   */
  connect(url?: string): Promise<void> {
    if (url) {
      this.url = url;
    }
    this.shouldReconnect = true;
    this.reconnectAttempts = 0;
    this.clearReconnectTimer();
    return this.internalConnect();
  }

  private internalConnect(): Promise<void> {
    return new Promise((resolve, reject) => {
      // Reject any existing pending requests
      for (const [, pending] of this.pending) {
        clearTimeout(pending.timer);
        pending.reject(new Error('Reconnecting'));
      }
      this.pending.clear();

      this.ws = new WebSocket(this.url);
      this.setStatus('connecting');

      this.ws.onopen = () => {
        this.setStatus('connected');
        this.reconnectAttempts = 0;
        // Send version handshake
        this.ws!.send(JSON.stringify({ version: API_VERSION }));
        // Resolve if connect() was called directly and handshake succeeded
        resolve();
      };

      this.ws.onmessage = (event: MessageEvent) => {
        try {
          const data = JSON.parse(event.data as string);

          // Check if it's a handshake error
          if (data.error && data.id === -1) {
            this.setStatus('error');
            reject(new Error(data.error));
            this.ws?.close();
            return;
          }

          // Check if it's an EventMessage (has 'type' but no 'id')
          if (data.type && data.id === undefined) {
            this.handleEvent(data);
            return;
          }

          // It's a CommandResponse - find and resolve the pending request
          const pending = this.pending.get(data.id);
          if (pending) {
            clearTimeout(pending.timer);
            this.pending.delete(data.id);
            pending.resolve(data as CommandResponse);
          }
        } catch (err) {
          console.error('Failed to parse WebSocket message:', err);
        }
      };

      this.ws.onerror = () => {
        this.setStatus('error');
        reject(new Error('WebSocket connection failed'));
      };

      this.ws.onclose = () => {
        this.setStatus('disconnected');
        // Reject all pending requests
        for (const [, pending] of this.pending) {
          clearTimeout(pending.timer);
          pending.reject(new Error('Connection closed'));
        }
        this.pending.clear();
        // Schedule reconnect
        this.scheduleReconnect();
      };

      // Timeout for connection + handshake
      setTimeout(() => {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
          this.setStatus('error');
          reject(new Error('Connection timeout'));
        }
      }, 5000);
    });
  }

  /**
   * Send a command and wait for a matching response.
   */
  async sendCommand(
    command: string,
    params?: Record<string, unknown>,
    timeoutMs: number = 10000
  ): Promise<CommandResponse> {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('Not connected');
    }

    const id = this.nextId++;
    const request: CommandRequest = {
      version: API_VERSION,
      id,
      command,
      params,
    };

    // Validate outgoing request against schema
    if (this.validateRequest) {
      const valid = this.validateRequest(request);
      if (!valid) {
        throw new Error(
          `Validation error: ${JSON.stringify(this.validateRequest.errors)}`
        );
      }
    }

    return new Promise<CommandResponse>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`Request ${id} timed out`));
      }, timeoutMs);

      this.pending.set(id, { resolve, reject, timer });
      this.ws!.send(JSON.stringify(request));
    });
  }

  /**
   * Register an event handler for a specific event type.
   */
  on(eventType: string, handler: (payload: unknown) => void): () => void {
    if (!this.eventHandlers.has(eventType)) {
      this.eventHandlers.set(eventType, new Set());
    }
    this.eventHandlers.get(eventType)!.add(handler);

    // Return unsubscribe function
    return () => {
      this.eventHandlers.get(eventType)?.delete(handler);
    };
  }

  /**
   * Disconnect from the WebSocket server and stop reconnecting.
   */
  disconnect(): void {
    this.shouldReconnect = false;
    this.clearReconnectTimer();
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.setStatus('disconnected');
  }

  /**
   * Load and compile the CommandRequest schema for client-side validation.
   */
  async loadSchema(schema: object): Promise<void> {
    this.validateRequest = this.ajv.compile(schema);
  }

  private handleEvent(data: { type: string; payload?: unknown }): void {
    const handlers = this.eventHandlers.get(data.type);
    if (handlers) {
      for (const handler of handlers) {
        try {
          handler(data.payload);
        } catch (err) {
          console.error('Event handler error:', err);
        }
      }
    }
  }
}
