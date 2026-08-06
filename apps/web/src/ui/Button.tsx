import type { ButtonHTMLAttributes } from "react";

type ButtonVariant = "primary" | "secondary" | "default" | "danger" | "link";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
};

export function Button({ variant = "default", className, children, ...rest }: ButtonProps) {
  return (
    <button className={`button button-${variant} ${className || ""}`} {...rest}>
      {children}
    </button>
  );
}
