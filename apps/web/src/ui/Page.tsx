import type { ReactNode, HTMLAttributes } from "react";

type DivProps = HTMLAttributes<HTMLDivElement>;

const Root: React.FC<DivProps & { maxWidth?: "lg" | "full" | string }> = ({ children, maxWidth, style, className, ...rest }) => {
  return <div className={`page ${className || ""}`} style={{ ...style, ...(maxWidth ? { maxWidth: maxWidth === "full" ? "100%" : maxWidth === "lg" ? "64rem" : maxWidth } : {}) }} {...rest}>{children}</div>;
};

const Header = ({ children, ...rest }: DivProps): React.JSX.Element => {
  return <header className={`page-header ${rest.className || ""}`} {...rest}>{children}</header>;
};

const Eyebrow = ({ children, ...rest }: DivProps): React.JSX.Element => {
  return <div className={`page-eyebrow ${rest.className || ""}`} {...rest}>{children}</div>;
};

const Title = ({ children, ...rest }: HTMLAttributes<HTMLHeadingElement>): React.JSX.Element => {
  return <h1 className={`page-title ${rest.className || ""}`} {...rest}>{children}</h1>;
};

const Description = ({ children, ...rest }: HTMLAttributes<HTMLParagraphElement>): React.JSX.Element => {
  return <p className={`page-description ${rest.className || ""}`} {...rest}>{children}</p>;
};

export type ContentProps = HTMLAttributes<HTMLDivElement> & {
  maxWidth?: string;
};

const Content: React.FC<DivProps & { maxWidth?: string }> = ({ children, maxWidth, style, className, ...rest }) => {
  return <div className={`page-content ${className || ""}`} style={{ ...style, ...(maxWidth ? { maxWidth } : {}) }} {...rest}>{children}</div>;
};

export type SectionProps = HTMLAttributes<HTMLElement> & {
  title?: string;
  description?: string;
};

const Section = ({ children, title, description, ...rest }: SectionProps): React.JSX.Element => {
  return (
    <section className={`page-section ${rest.className || ""}`} {...rest}>
      {title && <h2>{title}</h2>}
      {description && <p>{description}</p>}
      {children}
    </section>
  );
};

const Actions = ({ children, ...rest }: DivProps): React.JSX.Element => {
  return <div className={`page-actions ${rest.className || ""}`} {...rest}>{children}</div>;
};

export const Page = {
  Root,
  Header,
  Eyebrow,
  Title,
  Description,
  Content,
  Section,
  Actions,
};
