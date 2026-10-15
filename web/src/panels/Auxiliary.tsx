// Auxiliary —— 辅助区（256px 白底，可收起）。v3 起内容随视图映射（ui-baseline §五/八）：
//   - 数据看板与其他视图：「今日聚焦」日期卡 / 快速灵感 / 最近记录 / 项目进度（R4 起实数据）；
//   - 待办事项：月历 / 按属性看 / 按项目查看 / 即将到来（R5 起实数据）；
//   - 设置页：不渲染辅助区。
// R3 先落骨架结构与宽度，内容卡片在 R4/R5 充实实数据。
import type { ViewId } from '../state/app-store';
import { useStore } from '../state/app-store';

type AuxiliaryProps = {
  view: ViewId;
};

export function Auxiliary({ view }: AuxiliaryProps) {
  const { projects, todos, notes } = useStore();
  void projects;
  void todos;
  void notes;

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
        <>
          <div className="aux-card">
            <h2 className="aux-title">今日聚焦</h2>
            <div className="aux-placeholder">日期 / 快速灵感</div>
          </div>
          <div className="aux-card">
            <h2 className="aux-title">最近记录</h2>
            <div className="aux-placeholder">最近四条记录 · R4 开放</div>
          </div>
        </>
      )}
    </aside>
  );
}
