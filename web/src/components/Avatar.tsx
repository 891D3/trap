// 用户头像组件：展示本地头像图片（web/src/assets/avatar.png，已缩至 256px）。
// 侧栏与设置页共用，避免各自硬编码图片路径。
import avatarSrc from '../assets/avatar.png';

type AvatarProps = { name: string };

export function Avatar({ name }: AvatarProps) {
  return (
    <div className="avatar">
      <img src={avatarSrc} alt={`${name} 的头像`} draggable={false} />
    </div>
  );
}
