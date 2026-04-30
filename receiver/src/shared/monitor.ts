import {
  ACTIVEMQ_BASE_URL,
  VIRTUAL_TOPIC_NAME,
  BACKLOG_WARN_THRESHOLD,
  BACKLOG_CRITICAL_THRESHOLD,
  PROCESSOR_LABELS,
} from './constants.js';
import { getAuthHeader } from './auth.js';
import type { ProcessorType, QueueAlert, AlertLevel } from './types.js';

interface JolokiaResponse {
  status: number;
  value: unknown;
  error?: string;
}

interface QueueMBean {
  mbean: string;
  processor: ProcessorType | null;
}

function parseProcessor(mbean: string): ProcessorType | null {
  const match = mbean.match(/destinationName=Consumer\.([^.]+)\./);
  if (!match) return null;
  const name = match[1] as ProcessorType;
  return PROCESSOR_LABELS[name] ? name : null;
}

async function jolokiaPOST(body: object): Promise<JolokiaResponse | null> {
  try {
    const res = await fetch(`${ACTIVEMQ_BASE_URL}/api/jolokia/`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: getAuthHeader(),
      },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const text = await res.text().catch(() => '');
      console.log(`[monitor] POST → HTTP ${res.status}: ${text.slice(0, 200)}`);
      return null;
    }
    return (await res.json()) as JolokiaResponse;
  } catch (err) {
    console.log(`[monitor] POST 网络错误:`, err instanceof Error ? err.message : String(err));
    return null;
  }
}

async function findConsumerQueueMBeans(): Promise<QueueMBean[]> {
  const result = await jolokiaPOST({
    type: 'search',
    mbean: 'org.apache.activemq:type=Broker,destinationType=Queue,*',
  });

  if (!result) {
    console.log('[monitor] search POST 失败');
    return [];
  }

  if (result.status !== 200 || !Array.isArray(result.value)) {
    console.log(`[monitor] search 返回 status=${result.status}, error=${result.error}`);
    return [];
  }

  const all = result.value as string[];
  console.log(`[monitor] 搜索到 ${all.length} 个 Queue`);

  return all
    .filter((mbean) =>
      mbean.includes('Consumer.') &&
      mbean.includes(VIRTUAL_TOPIC_NAME) &&
      !mbean.includes('Advisory') &&
      !mbean.includes(',view=') &&
      !mbean.includes(',subType='),
    )
    .map((mbean) => ({
      mbean,
      processor: parseProcessor(mbean),
    }));
}

async function readQueueStats(mbean: string): Promise<{ queueSize: number; consumers: number; enqueueCount: number } | null> {
  const result = await jolokiaPOST({
    type: 'read',
    mbean,
    attribute: ['QueueSize', 'ConsumerCount', 'EnqueueCount'],
  });

  if (!result || result.status !== 200) return null;

  const v = result.value as Record<string, number>;
  return {
    queueSize: v.QueueSize ?? 0,
    consumers: v.ConsumerCount ?? 0,
    enqueueCount: v.EnqueueCount ?? 0,
  };
}

function computeAlert(processor: ProcessorType, queueSize: number, consumers: number, enqueueCount: number): QueueAlert {
  let level: AlertLevel;
  let message: string;

  if (consumers === 0 && queueSize > 0) {
    level = 'critical';
    message = `无消费者，${queueSize} 条堆积`;
  } else if (queueSize >= BACKLOG_CRITICAL_THRESHOLD) {
    level = 'critical';
    message = `堆积 ${queueSize} 条`;
  } else if (queueSize >= BACKLOG_WARN_THRESHOLD) {
    level = 'warn';
    message = `堆积 ${queueSize} 条`;
  } else {
    level = 'ok';
    message = queueSize > 0 ? `${queueSize} 条` : '空闲';
  }

  return {
    processor,
    label: PROCESSOR_LABELS[processor],
    level,
    message,
    stats: {
      enqueueCount,
      dequeueCount: 0,
      consumerCount: consumers,
      backlog: queueSize,
      timestamp: Date.now(),
    },
  };
}

export async function fetchQueueAlerts(): Promise<QueueAlert[]> {
  const queues = await findConsumerQueueMBeans();
  if (queues.length === 0) return [];

  const alerts: QueueAlert[] = [];
  for (const q of queues) {
    if (!q.processor) continue;
    const s = await readQueueStats(q.mbean);
    if (!s) continue;
    alerts.push(computeAlert(q.processor, s.queueSize, s.consumers, s.enqueueCount));
  }
  return alerts;
}
