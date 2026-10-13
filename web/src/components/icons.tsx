// 最小内联 SVG 图标集（stroke 风格，跟随 currentColor）。
// 不引入图标库依赖；后续如需大图集再评估。
import type { JSX } from 'react';

type IconProps = { size?: number };

function base(size: number, children: JSX.Element): JSX.Element {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {children}
    </svg>
  );
}

export const IconTodo = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <path d="M4 12.5 9.5 18 20 6.5" />
    </>
  ));

export const IconNote = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <path d="M6 3h12a1 1 0 0 1 1 1v13l-4 4H6a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1Z" />
      <path d="M15 21v-4h4" />
    </>
  ));

export const IconLink = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <path d="M10 14a5 5 0 0 0 7.1 0l2.4-2.4a5 5 0 0 0-7.1-7.1L11 5.9" />
      <path d="M14 10a5 5 0 0 0-7.1 0l-2.4 2.4a5 5 0 0 0 7.1 7.1L13 18.1" />
    </>
  ));

export const IconSettings = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <circle cx="12" cy="12" r="3.2" />
      <path d="M19 12a7 7 0 0 0-.1-1.2l2-1.5-2-3.4-2.4 1a7 7 0 0 0-2-1.2L14 3h-4l-.5 2.7a7 7 0 0 0-2 1.2l-2.4-1-2 3.4 2 1.5A7 7 0 0 0 5 12c0 .4 0 .8.1 1.2l-2 1.5 2 3.4 2.4-1a7 7 0 0 0 2 1.2L10 21h4l.5-2.7a7 7 0 0 0 2-1.2l2.4 1 2-3.4-2-1.5c.1-.4.1-.8.1-1.2Z" />
    </>
  ));

export const IconCollapse = ({ size = 16 }: IconProps) =>
  base(size, (
    <>
      <path d="m14 6-6 6 6 6" />
    </>
  ));

export const IconExpand = ({ size = 16 }: IconProps) =>
  base(size, (
    <>
      <path d="m10 6 6 6-6 6" />
    </>
  ));

export const IconPlus = ({ size = 16 }: IconProps) =>
  base(size, (
    <>
      <path d="M12 5v14M5 12h14" />
    </>
  ));

export const IconTrash = ({ size = 16 }: IconProps) =>
  base(size, (
    <>
      <path d="M4 7h16M9 7V4h6v3M6.5 7l1 13h9l1-13" />
    </>
  ));

export const IconCalendar = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <rect x="4" y="5" width="16" height="16" rx="2" />
      <path d="M4 10h16M8 3v4M16 3v4" />
    </>
  ));

export const IconSticky = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <path d="M5 4h14a1 1 0 0 1 1 1v9l-6 6H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1Z" />
      <path d="M14 20v-6h6" />
    </>
  ));

export const IconPulse = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <path d="M3 12h4l2.5-7 5 14 2.5-7h4" />
    </>
  ));

export const IconX = ({ size = 16 }: IconProps) =>
  base(size, (
    <>
      <path d="M6 6l12 12M18 6 6 18" />
    </>
  ));

export const IconArchive = ({ size = 16 }: IconProps) =>
  base(size, (
    <>
      <rect x="3" y="4" width="18" height="5" rx="1" />
      <path d="M5 9v10a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V9M10 13h4" />
    </>
  ));

export const IconExport = ({ size = 16 }: IconProps) =>
  base(size, (
    <>
      <path d="M12 3v12M7 10l5 5 5-5M4 21h16" />
    </>
  ));