// MainArea —— 主区域列（顶栏 + 主内容区）。v3 起顶栏抽到 Topbar 组件，
// v1 的独立标题说明与「今日完成」统计卡移除（数据看板视图承担该展示，R4）。
import type { ReactNode } from 'react';
import { Topbar } from './Topbar';
import type { ViewId } from '../state/app-store';

type MainAreaProps = {
  view: ViewId;
  children: ReactNode;
  /** 辅助区收起后：顶栏展开钮（v3 目检修正）。 */
  auxExpand?: { visible: boolean; onExpand: () => void };
};

export function MainArea({ view, children, auxExpand }: MainAreaProps) {
  return (
    <main className="main">
      <Topbar view={view} auxExpand={auxExpand} />
      {/* 主内容区：底色 --bg，p12/g12（ui-baseline §二）。 */}
      <section className="view">{children}</section>
    </main>
  );
}
