import {
  Activity,
  Bell,
  ChartNoAxesCombined,
  ClipboardCheck,
  Cloud,
  DollarSign,
  FileKey,
  Gauge,
  LayoutDashboard,
  Mail,
  PlusCircle,
  ScrollText,
  Server,
  Settings,
  ShieldCheck,
  Users
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

export type NavGroup = {
  label: string;
  items: NavItem[];
};

export type NavItem = {
  label: string;
  path: string;
  icon: LucideIcon;
};

export const navGroups: NavGroup[] = [
  {
    label: "资源运营",
    items: [
      { label: "概览", path: "/", icon: LayoutDashboard },
      { label: "实例管理", path: "/instances", icon: Server },
      { label: "创建实例", path: "/create", icon: PlusCircle },
      { label: "网络管理", path: "/network", icon: Gauge },
      { label: "自动化规则", path: "/automations", icon: Activity }
    ]
  },
  {
    label: "平台治理",
    items: [
      { label: "账号与密钥", path: "/profiles", icon: FileKey },
      { label: "任务中心", path: "/jobs", icon: ClipboardCheck },
      { label: "预算管理", path: "/budgets", icon: DollarSign },
      { label: "监控告警", path: "/monitoring", icon: ChartNoAxesCombined },
      { label: "审计日志", path: "/audit", icon: ScrollText },
      { label: "用户与权限", path: "/users", icon: Users }
    ]
  },
  {
    label: "OCI 服务",
    items: [
      { label: "邮件服务", path: "/email", icon: Mail }
    ]
  },
  {
    label: "系统",
    items: [
      { label: "安全护栏", path: "/guardrails", icon: ShieldCheck },
      { label: "通知设置", path: "/notifications", icon: Bell },
      { label: "系统设置", path: "/settings", icon: Settings }
    ]
  }
];

export const productMark = Cloud;

export const dashboardPath = "/";
