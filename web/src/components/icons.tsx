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

export const IconPanelLeftClose = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <rect x="3" y="4" width="18" height="16" rx="3" />
      <path d="M9.5 4v16" />
      <path d="m16.5 10-2.5 2 2.5 2" />
    </>
  ));

export const IconPanelLeftOpen = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <rect x="3" y="4" width="18" height="16" rx="3" />
      <path d="M9.5 4v16" />
      <path d="m12 10 2.5 2-2.5 2" />
    </>
  ));

export const IconDashboard = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <path d="M5 20V10M12 20V4M19 20v-6" />
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
export const IconSearch = ({ size = 16 }: IconProps) =>
  base(size, (
    <>
      <circle cx="11" cy="11" r="6.5" />
      <path d="m20 20-3.5-3.5" />
    </>
  ));

export const IconBell = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <path d="M18 9.5a6 6 0 1 0-12 0c0 4.5-1.5 5.5-1.5 5.5h15S18 14 18 9.5Z" />
      <path d="M10.3 19a2 2 0 0 0 3.4 0" />
    </>
  ));

export const IconFilter = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <path d="M4 5h16M7 12h10M10 19h4" />
    </>
  ));

export const IconMoon = ({ size = 18 }: IconProps) =>
  base(size, (
    <path d="M20 14.5A8.5 8.5 0 0 1 9.5 4 8.5 8.5 0 1 0 20 14.5Z" />
  ));

export const IconSun = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2m0 16v2M2 12h2m16 0h2M4.9 4.9l1.4 1.4m11.4 11.4 1.4 1.4M19.1 4.9l-1.4 1.4M6.3 17.7l-1.4 1.4" />
    </>
  ));

export const IconMonitor = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <rect x="3" y="4" width="18" height="13" rx="2" />
      <path d="M8 21h8m-4-4v4" />
    </>
  ));

export const IconStock = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <path d="M4 20V10m6 10V4m6 16v-7" />
    </>
  ));

export const IconDots = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <circle cx="5" cy="12" r="1.2" fill="currentColor" stroke="none" />
      <circle cx="12" cy="12" r="1.2" fill="currentColor" stroke="none" />
      <circle cx="19" cy="12" r="1.2" fill="currentColor" stroke="none" />
    </>
  ));

export const IconChevronDouble = ({ size = 16 }: IconProps) =>
  base(size, (
    <>
      <path d="m11 6 6 6-6 6M5 6l6 6-6 6" />
    </>
  ));

export const IconPencil = ({ size = 18 }: IconProps) =>
  base(size, (
    <>
      <path d="m15 5 4 4L8 20H4v-4L15 5Z" />
      <path d="m13 7 4 4" />
    </>
  ));

export const IconCode = ({ size = 16 }: IconProps) =>
  base(size, (
    <>
      <path d="m8.5 7-4 5 4 5" />
      <path d="m15.5 7 4 5-4 5" />
    </>
  ));
