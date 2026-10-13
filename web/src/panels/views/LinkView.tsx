import { useState } from 'react';
import { ConfirmDialog, InputDialog } from '../../components/dialogs';
import { IconPlus, IconTrash } from '../../components/icons';
import { avatarColorFor, hostOf, useStore, type QuickLink } from '../../state/app-store';

// T3：跳转动作（Open）依赖 Go 侧 QuickLinkService（T2），此处仅展示与增删 mock。
export function LinkView() {
  const { links, addLink, deleteLink } = useStore();
  const [adding, setAdding] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<QuickLink | null>(null);

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <button className="btn is-primary" onClick={() => setAdding(true)}>
          <IconPlus />
          添加入口
        </button>
      </div>

      {links.length === 0 ? (
        <div className="aux-placeholder" style={{ marginTop: 24 }}>
          <span>还没有快捷入口</span>
        </div>
      ) : (
        <div className="link-grid">
          {links.map((l) => (
            <div className="link-card" key={l.id} title={l.target}>
              {/* 图标块：10% 透明度浅底 + 同色字（模板风格） */}
              <div className="link-avatar" style={{ background: `${avatarColorFor(l.name)}1a`, color: avatarColorFor(l.name) }}>
                {l.name.charAt(0)}
              </div>
              <div style={{ minWidth: 0 }}>
                <div className="link-name">{l.name}</div>
                <div className="link-host">{l.kind === 'url' ? hostOf(l.target) : l.target}</div>
              </div>
              <button className="btn-icon" aria-label={`删除 ${l.name}`} onClick={() => setPendingDelete(l)}>
                <IconTrash />
              </button>
            </div>
          ))}
        </div>
      )}

      {adding && (
        <InputDialog
          title="添加快捷入口"
          fields={[
            { key: 'name', label: '名称', required: true, placeholder: '如：GitHub' },
            { key: 'target', label: 'URL 或本地路径', required: true, placeholder: 'https://…' },
            { key: 'groupName', label: '分组（可选）', placeholder: '如：开发' },
          ]}
          submitText="添加"
          onSubmit={(v) => {
            addLink(v.name?.trim() ?? '', v.target?.trim() ?? '', v.groupName?.trim() ?? '');
            setAdding(false);
          }}
          onCancel={() => setAdding(false)}
        />
      )}

      {pendingDelete && (
        <ConfirmDialog
          title="删除快捷入口"
          confirmText="删除"
          onConfirm={() => {
            deleteLink(pendingDelete.id);
            setPendingDelete(null);
          }}
          onCancel={() => setPendingDelete(null)}
        >
          确定删除「{pendingDelete.name}」？
        </ConfirmDialog>
      )}
    </div>
  );
}