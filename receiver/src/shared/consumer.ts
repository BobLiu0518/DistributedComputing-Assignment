import { ACTIVEMQ_BASE_URL, TOPIC_NAME, POLL_TIMEOUT_MS } from './constants.js';
import type { EmergencyMessage } from './types.js';

let authHeader = '';

export function setAuth(username: string, password: string): void {
  authHeader = `Basic ${Buffer.from(`${username}:${password}`).toString('base64')}`;
}

export async function verifyConnection(): Promise<{ ok: boolean; message: string }> {
  try {
    const res = await fetch(`${ACTIVEMQ_BASE_URL}/api/jolokia/`, {
      headers: { Authorization: authHeader },
    });
    if (res.status === 401 || res.status === 403) {
      return { ok: false, message: `认证失败 (HTTP ${res.status})` };
    }
    if (res.ok) {
      return { ok: true, message: '连接成功' };
    }
    return { ok: false, message: `异常响应 (HTTP ${res.status})` };
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err);
    return { ok: false, message: `连接失败: ${msg}` };
  }
}

export async function consumeOne(clientId: string): Promise<EmergencyMessage | null> {
  const url = `${ACTIVEMQ_BASE_URL}/api/message/${TOPIC_NAME}?type=topic&clientId=${encodeURIComponent(clientId)}&json=true&timeout=${POLL_TIMEOUT_MS}`;

  try {
    const res = await fetch(url, {
      headers: { Authorization: authHeader },
    });

    if (res.status === 204 || res.status === 404) {
      return null;
    }

    if (!res.ok) {
      console.error(`[consumer] HTTP ${res.status} from ActiveMQ`);
      return null;
    }

    const text = await res.text();
    if (!text || text.trim() === '') return null;

    return JSON.parse(text) as EmergencyMessage;
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err);
    console.error(`[consumer] 消费失败: ${msg}`);
    return null;
  }
}

export async function consumeLoop(
  clientId: string,
  onMessage: (msg: EmergencyMessage) => void,
  signal?: AbortSignal,
): Promise<void> {
  console.log(`[consumer] 开始订阅主题 ${TOPIC_NAME} (clientId=${clientId})`);

  while (!signal?.aborted) {
    try {
      const msg = await consumeOne(clientId);
      if (msg) {
        onMessage(msg);
      }
    } catch {
      await sleep(1000);
    }
  }

  console.log('[consumer] 消费循环已停止');
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
