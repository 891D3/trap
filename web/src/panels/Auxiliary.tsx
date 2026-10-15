// Auxiliary —— 辅助区容器（v3 §二/§八）：内容随视图映射。
//   待办事项：月历 / 按属性看占位卡（R5 充实实数据）；
//   其他视图：今日聚焦（日期卡/快速灵感/最近记录/项目进度，R4 起真实数据）；
//   设置页：不渲染（由 App 控制）。
import type { ViewId } from '../state/app-store';
import { TodayFocus } from './aux/TodayFocus';

type AuxiliaryProps = {
  view: ViewId;
};

export function Auxiliary({ view }: AuxiliaryProps) {
  return (
    <aside className="aux">
      {view === 'todo' ? (
        <>
          <div className="aux-card">
            <h2 className="aux-title">月历</h2>
            <div className="aux-placeholder">月历 · R5 开放</div>
          </div>
          <div className="aux-card">
            <h2 className="aux-title">按属性看</h2>
            <div className="aux-placeholder">分类筛选 · R5 开放</div>
          </div>
        </>
      ) : (
        <TodayFocus />
      )}
    </aside>
  );
}
