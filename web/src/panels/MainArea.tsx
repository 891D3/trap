import type { ReactNode } from 'react';

export type ViewMeta = { title: string; desc: string };

type MainAreaProps = {
  meta: ViewMeta;
  doneToday: number;
  showStat: boolean;
  showAuxToggle: boolean;
  auxOpen: boolean;
  onToggleAux: () => void;
  children: ReactNode;
};

export function MainArea({ meta, doneToday, showStat, showAuxToggle, auxOpen, onToggleAux, children }: MainAreaProps) {
  return (
    <main className="main">
      <header className="main-header">
        <div>
          <h1 className="main-title">{meta.title}</h1>
          <p className="main-desc">{meta.desc}</p>
        </div>
        {showAuxToggle && (
          <button className="btn" onClick={onToggleAux}>
            {auxOpen ? '关闭辅助区' : '辅助区'}
          </button>
        )}
      </header>

      {showStat && (
        <div className="stat-row">
          <div className="stat-card">
            <div className="stat-value">{doneToday}</div>
            <div className="stat-label">今日完成</div>
          </div>
        </div>
      )}

      <section className="view">{children}</section>
    </main>
  );
}