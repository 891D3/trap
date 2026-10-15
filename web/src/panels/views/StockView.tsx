// StockView —— 股票看台占位视图（PRD v2.0：导航项保留，视图 M2+ 开放）。
// 仅展示与 v3 设计一致的空态，不注册任何数据依赖。
export function StockView() {
  return (
    <div className="view-empty">
      <h2>股票看台</h2>
      <p>行情与持仓视图将在后续版本开放（M2+）。</p>
    </div>
  );
}
