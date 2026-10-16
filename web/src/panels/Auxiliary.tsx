// Auxiliary —— 辅助区容器（v3 §二/§八）：内容随视图映射。
//   待办事项：月历 / 按属性看占位卡（R5 充实实数据）；
//   其他视图：今日聚焦（日期卡/快速灵感/最近记录/项目进度，R4 起真实数据）；
//   设置页：不渲染（由 App 控制）。
import type { ViewId } from '../state/app-store';
import { TodoAux } from './aux/TodoAux';
import { TodayFocus } from './aux/TodayFocus';

type AuxiliaryProps = {
  view: ViewId;
  /** 当前是否收起（与左侧侧栏同套宽过渡动画）。 */
  collapsed?: boolean;
  /** 收起辅助区（App 层控制）。 */
  onCollapse?: () => void;
};

export function Auxiliary({ view, collapsed = false, onCollapse }: AuxiliaryProps) {
  return (
    <div className={`aux-rail${collapsed ? ' is-collapsed' : ''}`} aria-hidden={collapsed}>
      <aside className="aux">
        {/* v3：辅助区收起按钮（设计稿：收起钮在「今日聚焦」右侧；任何视图右上角） */}
        {onCollapse && (
          <button
            className="aux-collapse-btn"
            aria-label="收起辅助区"
            title="收起辅助区"
            onClick={onCollapse}
            style={{ position: 'absolute', top: 10, right: 10, zIndex: 5 }}
          >
            <AuxCollapseIcon />
          </button>
        )}
        {view === 'todo' ? <TodoAux /> : <TodayFocus />}
      </aside>
    </div>
  );
}

import { IconPanelLeftClose } from '../components/icons';
function AuxCollapseIcon(): React.JSX.Element {
  return <IconPanelLeftClose />;
}
