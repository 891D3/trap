import { useState } from 'react';
import { useTheme } from '../../state/app-store';
import { useStore } from '../../state/app-store';
import { Avatar } from '../../components/Avatar';

const THEME_OPTIONS: { value: 'system' | 'light' | 'dark'; label: string }[] = [
  { value: 'system', label: '跟随系统' },
  { value: 'light', label: '浅色' },
  { value: 'dark', label: '深色' },
];

// T3 骨架：姓名/主题本地可用；数据目录/导出目录等依赖 AppService（T2 接入）。
export function SettingsView() {
  const { user, renameUser } = useStore();
  const [theme, setTheme] = useTheme();
  const [nameDraft, setNameDraft] = useState(user.name);

  const saveName = () => {
    const name = nameDraft.trim();
    if (name) renameUser(name);
  };

  return (
    <div>
      <section className="settings-section">
        <h3>用户资料</h3>
        <div className="settings-row">
          <div className="settings-row-label">姓名</div>
          <div style={{ display: 'flex', gap: 8 }}>
            <input
              value={nameDraft}
              onChange={(e) => setNameDraft(e.target.value)}
              onBlur={saveName}
              onKeyDown={(e) => e.key === 'Enter' && saveName()}
              style={{
                padding: '7px 10px',
                border: '1px solid var(--border)',
                borderRadius: 'var(--radius-item)',
                background: 'var(--bg)',
                outline: 'none',
                width: 160,
              }}
            />
          </div>
        </div>
        <div className="settings-row">
          <div>
            <div className="settings-row-label">头像</div>
            <div className="settings-row-hint">选择本地图片（T2 接入，当前为默认头像）</div>
          </div>
          <Avatar name={user.name} />
        </div>
      </section>

      <section className="settings-section">
        <h3>主题</h3>
        <div className="settings-row">
          <div className="settings-row-label">外观</div>
          <div className="seg">
            {THEME_OPTIONS.map((o) => (
              <button
                key={o.value}
                className={`seg-btn${theme === o.value ? ' is-active' : ''}`}
                onClick={() => setTheme(o.value)}
              >
                {o.label}
              </button>
            ))}
          </div>
        </div>
      </section>

      <section className="settings-section">
        <h3>数据</h3>
        <div className="settings-row">
          <div>
            <div className="settings-row-label">数据目录</div>
            <div className="settings-row-hint">由 Go 侧提供（T2 接入）</div>
          </div>
          <button className="btn" disabled>
            打开目录
          </button>
        </div>
        <div className="settings-row">
          <div>
            <div className="settings-row-label">导出目录</div>
            <div className="settings-row-hint">信息记录导出 .md 的目标目录（T2 接入）</div>
          </div>
          <button className="btn" disabled>
            选择目录
          </button>
        </div>
      </section>

      <section className="settings-section">
        <h3>更多</h3>
        <div className="settings-row">
          <div>
            <div className="settings-row-label">AI 配置</div>
            <div className="settings-row-hint">M3 开放</div>
          </div>
          <button className="btn" disabled>
            配置
          </button>
        </div>
        <div className="settings-row">
          <div>
            <div className="settings-row-label">自动更新</div>
            <div className="settings-row-hint">M2 开放（mygo updater）</div>
          </div>
          <button className="btn" disabled>
            检查更新
          </button>
        </div>
      </section>
    </div>
  );
}