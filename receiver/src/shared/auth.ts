import { ACTIVEMQ_BASE_URL } from './constants.js';

let authHeader = '';

export function setAuth(username: string, password: string): void {
  authHeader = `Basic ${Buffer.from(`${username}:${password}`).toString('base64')}`;
}

export function getAuthHeader(): string {
  return authHeader;
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
