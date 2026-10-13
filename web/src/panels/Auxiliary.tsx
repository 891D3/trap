import { IconCalendar, IconPulse, IconSticky } from '../components/icons';

// T3 占位：右栏三块（日历/便签/消息流）均为 M1 开放；样式按 ui-baseline 的低权重设计。
export function Auxiliary() {
  return (
    <aside className="aux">
      <div className="aux-card">
        <h2 className="aux-title">日历</h2>
        <div className="aux-placeholder">
          <IconCalendar />
          <span>日历 · M1 开放</span>
        </div>
      </div>

      <div className="aux-card">
        <h2 className="aux-title">快捷便签</h2>
        <div className="aux-placeholder">
          <IconSticky />
          <span>便签 · M1 开放</span>
        </div>
      </div>

      <div className="aux-card">
        <h2 className="aux-title">动态消息流</h2>
        <div className="aux-placeholder">
          <IconPulse />
          <span>消息流 · M1 开放</span>
        </div>
      </div>
    </aside>
  );
}