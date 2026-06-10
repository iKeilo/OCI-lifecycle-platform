type StatusPillProps = {
  status: string;
};

const statusLabels: Record<string, string> = {
  Active: "运行中",
  Available: "可用",
  Draft: "草稿",
  Enabled: "已启用",
  Failed: "失败",
  Healthy: "正常",
  Limited: "受限",
  Normal: "正常",
  Paused: "已暂停",
  PENDING: "等待中",
  Provisioning: "创建中",
  Quiet: "无告警",
  RETRYING: "重试中",
  ROLLBACK_REQUIRED: "需要回滚",
  Running: "运行中",
  RUNNING: "执行中",
  Stopped: "已停止",
  Terminating: "正在终止",
  Terminated: "已终止",
  Success: "成功",
  SUCCESS: "成功",
  Warning: "警告",
  FAILED: "失败",
  CANCELLED: "已取消",
  MANUAL_REQUIRED: "需要人工处理",
  VERIFYING: "验证中",
  WAITING_OCI: "等待 OCI",
  "Waiting OCI": "等待 OCI"
};

export function StatusPill({ status }: StatusPillProps) {
  const tone = status.toLowerCase().replace(/\s+/g, "-");
  return <span className={`status-pill ${tone}`}>{statusLabels[status] ?? status}</span>;
}
