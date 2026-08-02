import * as React from "react";

/**
 * Square-cornered text field. The border is the only chrome; focus darkens it.
 */
export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  /** Small uppercase label above the field. */
  label?: string;
  placeholder?: string;
  /** "dark" = for use on black bands. */
  tone?: "light" | "dark";
}
export declare function Input(props: InputProps): JSX.Element;
