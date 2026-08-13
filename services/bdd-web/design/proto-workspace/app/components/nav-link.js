"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

// A plain nav <a> that gets `is-active` when its href matches the current
// route (see public/proto-extra.css for the paint). Used for every leaf row
// and, wrapped by CaretName, every row that also owns children.
// `disableActive`: for rows that all share one href (e.g. every Architecture
// outline row now points at plain "/architecture" — see sections.js) —
// without this they'd ALL light up together, which is more misleading than
// showing none of them lit.
export function NavLink({ href, children, className = "", disableActive = false, ...rest }) {
  const pathname = usePathname();
  const active = !disableActive && pathname === href;
  const cls = [className, active ? "is-active" : ""].filter(Boolean).join(" ");
  return (
    <Link href={href} className={cls || undefined} {...rest}>
      {children}
    </Link>
  );
}

// Every row that owns children (section roots + sub-groups) uses this:
// caret in a fixed slot toggles the <details>, the name itself navigates.
// stopPropagation keeps the name-click from also bubbling up to the
// <summary>'s native toggle behavior. `onNavigate`, if given, runs alongside
// (e.g. requesting an exact-line jump on the Architecture file view).
export function CaretName({ href, children, onNavigate, disableActive = false, ...rest }) {
  return (
    <>
      <span className="tree-caret" aria-hidden="true"></span>
      <NavLink
        className="tree-name-link"
        href={href}
        disableActive={disableActive}
        onClick={(e) => {
          e.stopPropagation();
          onNavigate?.();
        }}
        {...rest}
      >
        {children}
      </NavLink>
    </>
  );
}
