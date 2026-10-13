import type { FormEvent, ReactNode } from 'react';
import { createPortal } from 'react-dom';

type ConfirmDialogProps = {
  title: string;
  children: ReactNode;
  confirmText?: string;
  onConfirm: () => void;
  onCancel: () => void;
};

export function ConfirmDialog({ title, children, confirmText = '删除', onConfirm, onCancel }: ConfirmDialogProps) {
  return createPortal(
    <div className="overlay" onClick={onCancel}>
      <div className="dialog" onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <h2 className="dialog-title">{title}</h2>
        <p className="dialog-body">{children}</p>
        <div className="dialog-actions">
          <button className="btn" onClick={onCancel}>
            取消
          </button>
          <button
            className="btn"
            style={{ background: 'var(--danger)', borderColor: 'var(--danger)', color: '#fff' }}
            onClick={onConfirm}
          >
            {confirmText}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}

type InputDialogProps = {
  title: string;
  fields: { key: string; label: string; placeholder?: string; required?: boolean }[];
  submitText?: string;
  onSubmit: (values: Record<string, string>) => void;
  onCancel: () => void;
};

export function InputDialog({ title, fields, submitText = '确定', onSubmit, onCancel }: InputDialogProps) {
  const values = Object.fromEntries(fields.map((f) => [f.key, ''])) as Record<string, string>;

  const submit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    onSubmit(Object.fromEntries(fields.map((f) => [f.key, String(fd.get(f.key) ?? '')])));
  };

  return createPortal(
    <div className="overlay" onClick={onCancel}>
      <form className="dialog" onClick={(e) => e.stopPropagation()} onSubmit={submit} role="dialog" aria-modal="true">
        <h2 className="dialog-title">{title}</h2>
        {fields.map((f) => (
          <div className="field" key={f.key}>
            <label className="field-label" htmlFor={`dlg-${f.key}`}>
              {f.label}
            </label>
            <input id={`dlg-${f.key}`} name={f.key} placeholder={f.placeholder} required={f.required} autoFocus />
          </div>
        ))}
        <div className="dialog-actions">
          <button type="button" className="btn" onClick={onCancel}>
            取消
          </button>
          <button type="submit" className="btn is-primary">
            {submitText}
          </button>
        </div>
      </form>
    </div>,
    document.body,
  );
}