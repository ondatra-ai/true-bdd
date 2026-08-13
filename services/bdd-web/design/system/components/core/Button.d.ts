import * as React from "react";

/**
 * Large rectangular button — sharp corners, bold uppercase type, high contrast.
 */
export interface ButtonProps extends React.HTMLAttributes<HTMLElement> {
  children?: React.ReactNode;
  /** primary = black on white ink; secondary = outlined; ghost = borderless; inverse = for dark bands. */
  variant?: "primary" | "secondary" | "ghost" | "inverse";
  size?: "sm" | "md" | "lg";
  /** Stretch to fill its container (used inside a full ButtonRow). */
  full?: boolean;
  disabled?: boolean;
  /** Renders an <a> instead of a <button>. */
  href?: string;
  onClick?: React.MouseEventHandler;
}
export declare function Button(props: ButtonProps): JSX.Element;
