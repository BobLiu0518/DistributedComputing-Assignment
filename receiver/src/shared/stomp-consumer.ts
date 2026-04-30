import WebSocket from 'ws';
import { Client, type StompSubscription, type IFrame, type IMessage } from '@stomp/stompjs';
import type { EmergencyMessage } from './types.js';
import { STOMP_BROKER_URL } from './constants.js';

// Polyfill: @stomp/stompjs needs global WebSocket in Node.js
Object.assign(global, { WebSocket });

interface StompConsumerOptions {
  username: string;
  password: string;
  queueName: string;
  onMessage: (msg: EmergencyMessage) => void;
  onStatusChange?: (connected: boolean) => void;
}

export async function startStompConsumer(
  opts: StompConsumerOptions,
  signal: AbortSignal,
): Promise<void> {
  const destination = `/queue/${opts.queueName}`;

  const client = new Client({
    brokerURL: STOMP_BROKER_URL,
    connectHeaders: {
      login: opts.username,
      passcode: opts.password,
    },
    heartbeatIncoming: 10000,
    heartbeatOutgoing: 10000,
    reconnectDelay: 5000,

    onConnect: () => {
      console.log('[stomp] Connected');
      opts.onStatusChange?.(true);

      client.subscribe(
        destination,
        (message: IMessage) => {
          try {
            const body = JSON.parse(message.body) as EmergencyMessage;
            opts.onMessage(body);
          } catch {
            console.error('[stomp] Failed to parse message');
          }
        },
        { ack: 'auto' },
      );

      console.log(`[stomp] Subscribed to ${destination}`);
    },

    onStompError: (frame: IFrame) => {
      console.error('[stomp] Error:', frame.headers['message']);
    },

    onWebSocketClose: (evt: CloseEvent) => {
      console.log(`[stomp] WS closed (${evt.code})`);
      opts.onStatusChange?.(false);
    },
  });

  signal.addEventListener('abort', () => {
    console.log('[stomp] Shutting down...');
    client.deactivate();
  });

  console.log(`[stomp] Connecting to ${STOMP_BROKER_URL}...`);
  client.activate();

  // Wait for abort signal
  await new Promise<void>((resolve) => {
    signal.addEventListener('abort', () => resolve(), { once: true });
  });
}
