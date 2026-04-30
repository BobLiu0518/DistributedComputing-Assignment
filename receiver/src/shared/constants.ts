import type { EmergencyType, DispatchRule, ActionType, ProcessorType } from './types.js';

export const ACTIVEMQ_BASE_URL = 'https://mq.usst2.bobliu.tech';
export const STOMP_BROKER_URL = 'wss://mq.usst2.bobliu.tech/ws/';
export const VIRTUAL_TOPIC_NAME = 'VirtualTopic.campus.emergency';

export function consumerQueueName(processor: ProcessorType | 'dashboard'): string {
  return `Consumer.${processor}.${VIRTUAL_TOPIC_NAME}`;
}

export const MONITOR_INTERVAL_MS = 10_000;
export const BACKLOG_WARN_THRESHOLD = 500;
export const BACKLOG_CRITICAL_THRESHOLD = 2000;

export const DASHBOARD_PORT = 3456;
export const DASHBOARD_URL = `http://localhost:${DASHBOARD_PORT}`;

export const PROCESSOR_LABELS: Record<ProcessorType, string> = {
  'gate-controller': '闸机控制器',
  'sms-sender': '短信发送器',
  'alarm-controller': '警报控制器',
  'power-controller': '电源控制器',
  'valve-controller': '水阀控制器',
  'medical-dispatcher': '医疗调度器',
};

export const PROCESSOR_ACTIONS: Record<ProcessorType, ActionType[]> = {
  'gate-controller': ['gate_open', 'gate_lock'],
  'sms-sender': ['sms_all', 'sms_security', 'sms_medical'],
  'alarm-controller': ['alarm'],
  'power-controller': ['power_cut'],
  'valve-controller': ['valve_close'],
  'medical-dispatcher': ['medical_dispatch'],
};

export const ACTION_LABELS: Record<ActionType, string> = {
  alarm: '启动警报',
  gate_open: '闸机全开',
  gate_lock: '闸机锁死',
  power_cut: '切断电源',
  sms_all: '全校短信通知',
  sms_security: '短信通知保安',
  sms_medical: '短信通知医务室',
  valve_close: '关闭水阀',
  medical_dispatch: '医疗调度',
};

export const ACTION_DELAYS: Partial<Record<ActionType, number>> = {
  alarm: 1500,
  gate_open: 1000,
  gate_lock: 800,
  power_cut: 2000,
  sms_all: 1000,
  sms_security: 800,
  sms_medical: 800,
  valve_close: 2000,
  medical_dispatch: 1200,
};

export const DISPATCH_RULES: Record<EmergencyType, DispatchRule> = {
  火灾: {
    actions: [
      { type: 'alarm', label: '启动火灾警报' },
      { type: 'gate_open', label: '闸机全开（疏散通道）' },
    ],
  },
  燃气泄漏: {
    actions: [
      { type: 'power_cut', label: '切断燃气区域电源' },
      { type: 'sms_all', label: '发送疏散短信' },
    ],
  },
  设备过热: {
    actions: [
      { type: 'alarm', label: '启动过热警报' },
      { type: 'power_cut', label: '切断故障设备电源' },
    ],
  },
  爆炸: {
    actions: [
      { type: 'alarm', label: '启动爆炸警报' },
      { type: 'sms_all', label: '全校紧急短信' },
      { type: 'medical_dispatch', label: '医疗队紧急调度' },
    ],
  },
  恐怖袭击: {
    actions: [
      { type: 'alarm', label: '启动恐袭警报' },
      { type: 'gate_open', label: '全校闸机全开' },
      { type: 'sms_all', label: '全校紧急短信' },
    ],
  },
  水管爆裂: {
    actions: [
      { type: 'valve_close', label: '关闭区域水阀' },
    ],
  },
  非法入侵: {
    actions: [
      { type: 'alarm', label: '启动入侵警报' },
      { type: 'gate_lock', label: '锁定相关区域闸机' },
      { type: 'sms_security', label: '通知保安赶赴现场' },
    ],
  },
  人员伤亡: {
    actions: [
      { type: 'medical_dispatch', label: '医疗队紧急调度' },
      { type: 'sms_medical', label: '通知医务室准备急救' },
    ],
  },
  生化泄漏: {
    actions: [
      { type: 'alarm', label: '启动生化泄漏警报' },
      { type: 'power_cut', label: '切断实验室电源' },
      { type: 'sms_all', label: '发送疏散+防护短信' },
    ],
  },
  打架斗殴: {
    actions: [
      { type: 'sms_security', label: '短信通知保安到场' },
    ],
  },
  食物中毒: {
    actions: [
      { type: 'sms_medical', label: '通知医务室准备救治' },
      { type: 'medical_dispatch', label: '医疗队赶赴食堂' },
    ],
  },
  电梯困人: {
    actions: [
      { type: 'sms_security', label: '通知保安联系电梯维保' },
    ],
  },
  踩踏事件: {
    actions: [
      { type: 'alarm', label: '启动踩踏警报' },
      { type: 'medical_dispatch', label: '医疗队紧急调度' },
      { type: 'sms_all', label: '发送疏散引导短信' },
    ],
  },
};
