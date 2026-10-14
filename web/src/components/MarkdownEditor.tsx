import { useEditor, Milkdown, MilkdownProvider } from '@milkdown/react';
import { Crepe } from '@milkdown/crepe';
import '@milkdown/crepe/theme/common/style.css';
import '@milkdown/crepe/theme/frame.css';
import { useRef } from 'react';

import './markdown-editor.css';

type MarkdownEditorProps = {
  value: string;
  onChange: (markdown: string) => void;
};

/**
 * T5 的 GFM Markdown 编辑器。
 *
 * 以 key 绑定当前记录后，切换记录会创建新的 Crepe 实例；编辑期间的
 * markdownUpdated 只向上层报告内容，不直接触碰 store 或 IPC。
 */
export function MarkdownEditor({ value, onChange }: MarkdownEditorProps) {
  return (
    <MilkdownProvider>
      <MarkdownEditorInstance value={value} onChange={onChange} />
    </MilkdownProvider>
  );
}

function MarkdownEditorInstance({ value, onChange }: MarkdownEditorProps) {
  const initialValue = useRef(value);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  useEditor(
    (root) => {
      const crepe = new Crepe({
        root,
        defaultValue: initialValue.current,
        features: {
          [Crepe.Feature.AI]: false,
        },
      });
      crepe.on((listener) => {
        listener.markdownUpdated((_ctx, markdown) => {
          onChangeRef.current(markdown);
        });
      });
      return crepe;
    },
    [],
  );

  return <div className="markdown-editor"><Milkdown /></div>;
}
