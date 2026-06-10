import { Bell, CheckCheck, MailWarning, ShieldAlert } from "lucide-react";
import { useEffect, useState } from "react";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import { listNotifications, markAllNotificationsRead, markNotificationRead } from "../services/api";
import type { Notification } from "../services/api";

export function NotificationsPage() {
  const [items, setItems] = useState<Notification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [errorMessage, setErrorMessage] = useState("");

  async function load() {
    setIsLoading(true);
    setErrorMessage("");
    try {
      const response = await listNotifications();
      setItems(response.items);
      setUnreadCount(response.unreadCount);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "加载通知失败");
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function markRead(id: string) {
    await markNotificationRead(id);
    await load();
  }

  async function markAllRead() {
    await markAllNotificationsRead();
    await load();
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="消息"
        title="站内通知"
        description="查看任务、凭据和系统事件通知；敏感通知只在站内和已配置邮件渠道中展示。"
        actions={
          <button className="secondary-button" onClick={() => void markAllRead()} disabled={unreadCount === 0}>
            <CheckCheck size={18} />
            全部已读
          </button>
        }
      />

      <AsyncState isLoading={isLoading} error={errorMessage} empty={!isLoading && !errorMessage && items.length === 0} emptyText="暂无通知" />

      {!isLoading && !errorMessage ? (
        <section className="notification-list">
          {items.map((item) => (
            <article className={`notification-card glass-panel ${item.read ? "read" : "unread"} ${item.severity}`} key={item.id}>
              <div className="notification-icon">
                {item.sensitive ? <ShieldAlert size={20} /> : <Bell size={20} />}
              </div>
              <div className="notification-body">
                <div className="notification-title-row">
                  <h2>{item.title}</h2>
                  <span>{formatTime(item.createdAt)}</span>
                </div>
                <pre>{item.message}</pre>
                <div className="notification-meta">
                  <span>{item.category}</span>
                  {item.resourceType ? <span>{item.resourceType}: {item.resourceId}</span> : null}
                  {item.sensitive ? <span className="sensitive-chip">敏感</span> : null}
                  {item.emailRequested ? (
                    <span className={item.emailSent ? "email-chip sent" : "email-chip failed"}>
                      {item.emailSent ? "邮件已发送" : `邮件未发送${item.emailError ? `：${item.emailError}` : ""}`}
                    </span>
                  ) : null}
                </div>
              </div>
              {!item.read ? (
                <button className="icon-button bordered" aria-label="标记已读" onClick={() => void markRead(item.id)}>
                  <CheckCheck size={18} />
                </button>
              ) : (
                <MailWarning size={18} className="muted-icon" />
              )}
            </article>
          ))}
        </section>
      ) : null}
    </div>
  );
}

function formatTime(value: string) {
  if (!value) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(value));
}
