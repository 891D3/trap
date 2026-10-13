import { useEffect, useState } from 'react';
import { toChineseError } from '../../api/app-settings';
import { Avatar } from '../../components/Avatar';
import { useStore, useTheme, type ThemeSetting } from '../../state/app-store';

const THEME_OPTIONS: { value: ThemeSetting; label: string }[] = [
  { value: 'system', label: '跟随系统' },
  { value: 'light', label: '浅色' },
  { value: 'dark', label: '深色' },
];

/** T3.1 设置页：数据来自 Go AppService，组件不直接连接 mygo IPC/Event。 */
export function SettingsView() {
  const {
    user,
    renameUser,
    exportDir,
    dataDir,
    settingsLoading,
    settingsError,
    setExportDir,
  } = useStore();
  const [theme, setTheme] = useTheme();
  const [nameDraft, setNameDraft] = useState(user.name);
  const [exportDraft, setExportDraft] = useState(exportDir);
  const [saving, setSaving] = useState<'name' | 'theme' | 'export' | null>(null);
  const [localError, setLocalError] = useState<string | null>(null);

  // Go 设置加载完成或事件同步后，把服务端姓名同步到输入框。
  useEffect(() => {
    setNameDraft(user.name);
  }, [user.name]);

  // 导出目录由 Go 侧事件/加载结果驱动，避免保留旧的本地 mock 值。
  useEffect(() => {
    setExportDraft(exportDir);
  }, [exportDir]);

  const saveName = async (): Promise<void> => {
    const name = nameDraft.trim();
    if (!name) {
      setLocalError('姓名不能为空');
      return;
    }
    setLocalError(null);
    setSaving('name');
    try {
      await renameUser(name);
    } catch (error: unknown) {
      setLocalError(toChineseError(error, '用户资料保存失败，请稍后重试'));
    } finally {
      setSaving(null);
    }
  };

  const saveTheme = async (nextTheme: ThemeSetting): Promise<void> => {
    setLocalError(null);
    setSaving('theme');
    try {
      await setTheme(nextTheme);
    } catch (error: unknown) {
      setLocalError(toChineseError(error, '主题保存失败，请稍后重试'));
    } finally {
      setSaving(null);
    }
  };

  const saveExportDirectory = async (): Promise<void> => {
    const directory = exportDraft.trim();
    if (directory === exportDir) return;
    setLocalError(null);
    setSaving('export');
    try {
      await setExportDir(directory);
    } catch (error: unknown) {
      setLocalError(toChineseError(error, '导出目录保存失败，请稍后重试'));
    } finally {
      setSaving(null);
    }
  };

  const visibleError = localError ?? settingsError;

  return (
    <div>
      {settingsLoading && <div className="settings-row-hint">正在读取本地设置…</div>}
      {visibleError && (
        <div
          role="alert"
          style={{
            marginBottom: 16,
            padding: '8px 10px',
            borderRadius: 'var(--radius-item)',
            color: 'var(--danger)',
            background: 'var(--danger-soft)',
          }}
        >
          {visibleError}
        </div>
      )}

      <section className="settings-section">
        <h3>用户资料</h3>
        <div className="settings-row">
          <div className="settings-row-label">姓名</div>
          <div style={{ display: 'flex', gap: 8 }}>
            <input
              value={nameDraft}
              disabled={settingsLoading || saving !== null}
              onChange={(e) => setNameDraft(e.target.value)}
              onBlur={() => void saveName()}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  void saveName();
                }
              }}
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
            <div className="settings-row-hint">选择本地图片（后续版本接入，当前为默认头像）</div>
          </div>
          <Avatar name={user.name} />
        </div>
      </section>

      <section className="settings-section">
        <h3>主题</h3>
        <div className="settings-row">
          <div className="settings-row-label">外观</div>
          <div className="seg">
            {THEME_OPTIONS.map((option) => (
              <button
                key={option.value}
                className={`seg-btn${theme === option.value ? ' is-active' : ''}`}
                disabled={settingsLoading || saving !== null}
                onClick={() => void saveTheme(option.value)}
              >
                {option.label}
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
            <div className="settings-row-hint">Go 侧 SQLite 数据目录（只读）</div>
          </div>
          <input
            aria-label="数据目录"
            value={settingsLoading ? '读取中…' : dataDir || '未返回数据目录'}
            readOnly
            disabled
            style={{
              minWidth: 220,
              padding: '7px 10px',
              border: '1px solid var(--border)',
              borderRadius: 'var(--radius-item)',
              background: 'var(--bg)',
              color: 'var(--text-secondary)',
            }}
          />
        </div>
        <div className="settings-row">
          <div>
            <div className="settings-row-label">导出目录</div>
            <div className="settings-row-hint">信息记录导出 .md 的目标目录（可手动填写）</div>
          </div>
          <input
            aria-label="导出目录"
            value={exportDraft}
            placeholder="例如：/Users/你的用户名/Documents/Trap"
            disabled={settingsLoading || saving !== null}
            onChange={(e) => setExportDraft(e.target.value)}
            onBlur={() => void saveExportDirectory()}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                void saveExportDirectory();
              }
            }}
            style={{
              minWidth: 220,
              padding: '7px 10px',
              border: '1px solid var(--border)',
              borderRadius: 'var(--radius-item)',
              background: 'var(--bg)',
              outline: 'none',
            }}
          />
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
