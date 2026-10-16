// OnboardingView —— 首次安装初始化页（Onboarding D3；视觉按用户参考图）：
//   点阵底图 + 居中白卡（人形图标圈 / 标题 / 副标题 / 表单 / 全宽主按钮）。
//   表单：用户名（必填）、导出目录（原生选择器，默认 ~/.trap/exports）、
//   主题偏好（浅/深/跟随系统）、数据目录只读展示 + 打开目录。
// 完成后 store.appInitialized=true → App 切主界面（D4）。
import { useEffect, useRef, useState } from 'react';
import { pickDirectory, openDataDir, toChineseError } from '../api/app-settings';
import { useStore } from '../state/app-store';

const THEMES: { value: 'system' | 'light' | 'dark'; label: string }[] = [
  { value: 'system', label: '跟随系统' },
  { value: 'light', label: '浅色' },
  { value: 'dark', label: '深色' },
];

export function OnboardingView(): React.JSX.Element {
  const { completeOnboarding, dataDir, theme } = useStore();

  const [name, setName] = useState('');
  const [exportDir, setExportDir] = useState('');
  const [themePick, setThemePick] = useState<'system' | 'light' | 'dark'>(theme);
  const [submitting, setSubmitting] = useState(false);
  const [localError, setLocalError] = useState<string | null>(null);
  const nameRef = useRef<HTMLInputElement>(null);

  // 首屏聚焦用户名输入。
  useEffect(() => { nameRef.current?.focus(); }, []);

  const pickDir = async (): Promise<void> => {
    try {
      const dir = await pickDirectory();
      if (dir) setExportDir(dir);
    } catch (error: unknown) {
      setLocalError(toChineseError(error, '目录选择失败，请重试'));
    }
  };

  const openDir = async (): Promise<void> => {
    try {
      await openDataDir();
    } catch (error: unknown) {
      setLocalError(toChineseError(error, '打开数据目录失败'));
    }
  };

  const submit = async (): Promise<void> => {
    if (!name.trim()) {
      setLocalError('请输入用户名');
      return;
    }
    if (submitting) return; // 防重复提交
    setSubmitting(true);
    setLocalError(null);
    try {
      await completeOnboarding({ name: name.trim(), exportDir: exportDir.trim(), theme: themePick });
      // 完成后 store.appInitialized 翻转 → App 自动切主界面。
    } catch (error: unknown) {
      setLocalError(toChineseError(error, '初始化失败，请重试'));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="onboarding">
      <div className="onboarding-card">
        <div className="onboarding-avatar">
          <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <circle cx="12" cy="8" r="4" />
            <path d="M4 21c0-4 3.6-7 8-7s8 3 8 7" />
          </svg>
        </div>
        <h1 className="onboarding-title">欢迎使用 Trap</h1>
        <p className="onboarding-sub">输入你的名字开始使用个人工作台</p>

        <div className="onboarding-form">
          <label className="onboarding-label" htmlFor="ob-name">用户名</label>
          <input
            id="ob-name"
            aria-label="用户名"
            ref={nameRef}
            className="onboarding-input"
            placeholder="怎么称呼你？"
            value={name}
            maxLength={30}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                void submit();
              }
            }}
          />

          <label className="onboarding-label" htmlFor="ob-export">导出目录（可选）</label>
          <div className="onboarding-dir-row">
            <input
              id="ob-export"
              className="onboarding-input"
              placeholder="默认 ~/.trap/exports"
              value={exportDir}
              onChange={(e) => setExportDir(e.target.value)}
            />
            <button className="btn onboarding-dir-btn" onClick={() => void pickDir()}>选择…</button>
          </div>

          <span className="onboarding-label">主题偏好</span>
          <div className="onboarding-theme-row" role="radiogroup" aria-label="主题偏好">
            {THEMES.map((t) => (
              <button
                key={t.value}
                className={`onboarding-theme${themePick === t.value ? ' is-active' : ''}`}
                role="radio"
                aria-checked={themePick === t.value}
                onClick={() => setThemePick(t.value)}
              >
                {t.label}
              </button>
            ))}
          </div>

          <span className="onboarding-label">数据目录</span>
          <div className="onboarding-dir-row">
            <input
              className="onboarding-input"
              aria-label="数据目录"
              readOnly
              value={dataDir || '~/.trap'}
            />
            <button className="btn onboarding-dir-btn" onClick={() => void openDir()}>打开目录</button>
          </div>

          {localError && <div role="alert" className="onboarding-error">{localError}</div>}

          <button
            className="btn is-primary onboarding-submit"
            disabled={submitting || !name.trim()}
            onClick={() => void submit()}
          >
            {submitting ? '初始化中…' : '开始使用'}
          </button>
        </div>
      </div>
    </div>
  );
}
