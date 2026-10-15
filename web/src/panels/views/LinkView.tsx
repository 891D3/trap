// LinkView —— v3 快捷入口视图（PRD v2.0 §4.5 / ui-baseline §5.4）：
//   卡片网格（每行 3–4 个），分组为分节标题；点击卡片经 Go 白名单命令打开；
//   添加/编辑弹层（名称/URL 或本地路径/分组），删除二次确认。
// 数据全部走真实 QuickLinkService（v1 的 mock 已在 R6 移除）。
import { useMemo, useState } from 'react';
import { toChineseError } from '../../api/app-settings';
import { hostOf, useStore, type QuickLink } from '../../state/app-store';
import { ConfirmDialog, InputDialog } from '../../components/dialogs';
import { IconPencil, IconPlus, IconTrash } from '../../components/icons';

export function LinkView() {
  const { links, linksLoading, linksError, addLink, editLink, deleteLink, openLink } = useStore();
  const [draft, setDraft] = useState<{ id: number | null; name: string; target: string; groupName: string } | null>(null);
  const [pendingDelete, setPendingDelete] = useState<QuickLink | null>(null);
  const [busyId, setBusyId] = useState<number | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  /** 分组（group_name 空串=未分组，排最后）。 */
  const groups = useMemo(() => {
    const map = new Map<string, QuickLink[]>();
    for (const l of links) {
      const key = l.group || '未分组';
      const list = map.get(key) ?? [];
      list.push(l);
      map.set(key, list);
    }
    return [...map.entries()];
  }, [links]);

  const submitDraft = async (): Promise<void> => {
    if (!draft) return;
    const name = draft.name.trim();
    const target = draft.target.trim();
    const groupName = draft.groupName.trim();
    if (!name || !target) return; // InputDialog required 已校验，此处兜底
    setActionError(null);
    try {
      if (draft.id === null) {
        await addLink(name, target, groupName);
      } else {
        const current = links.find((l) => l.id === draft.id);
        await editLink(draft.id, name, target, groupName, current?.sortOrder ?? 0);
      }
      setDraft(null);
    } catch (error: unknown) {
      setActionError(toChineseError(error, '保存失败'));
    }
  };

  const clickLink = async (link: QuickLink): Promise<void> => {
    setBusyId(link.id);
    setActionError(null);
    try {
      await openLink(link.id);
    } catch (error: unknown) {
      setActionError(toChineseError(error, '打开失败，请检查目标是否有效'));
    } finally {
      setBusyId(null);
    }
  };

  const removeLinkLocal = async (): Promise<void> => {
    if (!pendingDelete) return;
    setActionError(null);
    try {
      await deleteLink(pendingDelete.id);
      setPendingDelete(null);
    } catch (error: unknown) {
      setActionError(toChineseError(error, '删除失败'));
    }
  };

  return (
    <div className="linkv2">
      {actionError && (
        <div role="alert" className="error-box">{actionError}</div>
      )}

      <div className="link-toolbar">
        <button className="btn is-primary" onClick={() => setDraft({ id: null, name: '', target: '', groupName: '' })}>
          <IconPlus />添加入口
        </button>
      </div>

      {linksLoading ? (
        <div className="aux-empty">正在读取快捷入口…</div>
      ) : (
        groups.map(([group, items]) => (
          <section key={group} className="link-group">
            <h2 className="link-group-title">{group}</h2>
            <div className="link-grid v3">
              {items.map((l) => (
                <div className="link-card v3" key={l.id} title={l.target}>
                  <button className="link-card-main" onClick={() => void clickLink(l)} disabled={busyId !== null}>
                    {/* 图标块：首字母本地图标（v1.1 决策：M0 不抓 favicon，避免主动联网） */}
                    <span className={`link-avatar kind-${l.kind}`}>{l.name.charAt(0)}</span>
                    <span className="link-text">
                      <span className="link-name">{l.name}</span>
                      <span className="link-host">{l.kind === 'url' ? hostOf(l.target) : l.target}</span>
                    </span>
                  </button>
                  <div className="link-card-actions">
                    <button
                      className="btn-icon"
                      aria-label={`编辑 ${l.name}`}
                      onClick={() => setDraft({ id: l.id, name: l.name, target: l.target, groupName: l.group || '' })}
                    >
                      <IconPencil size={14} />
                    </button>
                    <button className="btn-icon" aria-label={`删除 ${l.name}`} onClick={() => setPendingDelete(l)}>
                      <IconTrash size={14} />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </section>
        ))
      )}
      {links.length === 0 && !linksLoading && (
        <div className="aux-empty">还没有快捷入口，点「添加入口」创建</div>
      )}
      {linksError && !actionError && <div className="aux-empty">{linksError}</div>}

      {draft && (
        <InputDialog
          title={draft.id === null ? '添加快捷入口' : '编辑快捷入口'}
          fields={[
            { key: 'name', label: '名称', required: true, placeholder: '如：GitHub' },
            { key: 'target', label: 'URL 或本地路径', required: true, placeholder: 'https://… 或 /Applications/…' },
            { key: 'groupName', label: '分组（可选）', placeholder: '如：开发' },
          ]}
          submitText={draft.id === null ? '添加' : '保存'}
          onSubmit={(v) => {
            setDraft({ ...draft, name: v.name ?? '', target: v.target ?? '', groupName: v.groupName ?? '' });
            void submitDraft();
          }}
          onCancel={() => setDraft(null)}
        />
      )}

      {pendingDelete && (
        <ConfirmDialog
          title="删除快捷入口"
          confirmText="删除"
          onConfirm={() => void removeLinkLocal()}
          onCancel={() => setPendingDelete(null)}
        >
          确定删除「{pendingDelete.name}」？
        </ConfirmDialog>
      )}
    </div>
  );
}
