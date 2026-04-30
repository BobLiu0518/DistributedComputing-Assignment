import type { EmergencyAction, EmergencyMessage, NodeRole } from '../shared/types.js';
import { ACTION_DELAYS } from '../shared/constants.js';

let _actionSeq = 0;
function nextActionId(): string {
  return `action-${Date.now()}-${++_actionSeq}`;
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export async function executeAction(
  action: EmergencyAction,
  onStatusChange?: (action: EmergencyAction) => void,
): Promise<EmergencyAction> {
  action.status = 'running';
  action.startedAt = Date.now();
  onStatusChange?.({ ...action });

  const baseMs = ACTION_DELAYS[action.type] ?? 1000;
  const ms = baseMs + Math.floor(Math.random() * baseMs * 0.5);

  try {
    await delay(ms);
    if (Math.random() < 0.05) {
      throw new Error('设备响应超时');
    }
    action.status = 'done';
    action.finishedAt = Date.now();
  } catch (err: unknown) {
    action.status = 'error';
    action.finishedAt = Date.now();
    action.error = err instanceof Error ? err.message : String(err);
  }

  onStatusChange?.({ ...action });
  return action;
}

export function createActionsForMessage(
  msg: EmergencyMessage,
  node: NodeRole,
  actions: { type: EmergencyAction['type']; label: string }[],
): EmergencyAction[] {
  return actions.map((a) => ({
    id: nextActionId(),
    type: a.type,
    label: a.label,
    node,
    message: msg,
    status: 'pending' as const,
    startedAt: 0,
    finishedAt: null,
  }));
}
