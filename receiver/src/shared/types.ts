export type EmergencyType =
  | '火灾'
  | '燃气泄漏'
  | '设备过热'
  | '爆炸'
  | '恐怖袭击'
  | '水管爆裂'
  | '非法入侵'
  | '人员伤亡'
  | '生化泄漏'
  | '打架斗殴'
  | '食物中毒'
  | '电梯困人'
  | '踩踏事件';

export interface EmergencyMessage {
  type: EmergencyType;
  location: string;
  timestamp: number;
  seq: number;
}

export type NodeRole = 'security' | 'medical' | 'dashboard';

export type ActionType =
  | 'alarm'
  | 'gate_open'
  | 'gate_lock'
  | 'power_cut'
  | 'sms_all'
  | 'sms_security'
  | 'sms_medical'
  | 'valve_close'
  | 'medical_dispatch';

export interface EmergencyAction {
  id: string;
  type: ActionType;
  label: string;
  node: NodeRole;
  message: EmergencyMessage;
  status: 'pending' | 'running' | 'done' | 'error';
  startedAt: number;
  finishedAt: number | null;
  error?: string;
}

export interface NodeReport {
  node: NodeRole;
  nodeLabel: string;
  action: EmergencyAction;
}

export interface DispatchRule {
  nodes: NodeRole[];
  actions: { type: ActionType; label: string }[];
}
