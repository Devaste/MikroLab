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

/**
 * WebSocket client for the MikroLab API.
 *
 * Usage:
 *   const client = new ApiClient();
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

  constructor() {
    this.ajv = new Ajv();
  }

  /**
   * Connect to the WebSocket server and perform the version handshake.
   */
  connect(url: string = WS_URL): Promise<void> {
    return new Promise((resolve, reject) => {
      this.ws = new WebSocket(url);

      this.ws.onopen = () => {
        // Send version handshake
        this.ws!.send(JSON.stringify({ version: API_VERSION }));
      };

      this.ws.onmessage = (event: MessageEvent) => {
        try {
          const data = JSON.parse(event.data as string);

          // Check if it's a handshake error
          if (data.error && data.id === -1) {
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

      this.ws.onerror = (event: Event) => {
        reject(new Error('WebSocket connection failed'));
      };

      this.ws.onclose = () => {
        // Reject all pending requests
        for (const [, pending] of this.pending) {
          clearTimeout(pending.timer);
          pending.reject(new Error('Connection closed'));
        }
        this.pending.clear();
      };

      // Timeout for connection + handshake
      setTimeout(() => {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
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
   * Disconnect from the WebSocket server.
   */
  disconnect(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
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