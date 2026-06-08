import { AlertCircle, Loader2 } from "lucide-react";

type AsyncStateProps = {
  isLoading: boolean;
  error: string;
  empty?: boolean;
  emptyText?: string;
};

export function AsyncState({ isLoading, error, empty = false, emptyText = "暂无数据" }: AsyncStateProps) {
  if (isLoading) {
    return (
      <div className="async-state">
        <Loader2 size={18} className="spin" />
        <span>正在加载数据...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="async-state error">
        <AlertCircle size={18} />
        <span>{error}</span>
      </div>
    );
  }

  if (empty) {
    return <div className="async-state">{emptyText}</div>;
  }

  return null;
}
