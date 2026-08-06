import type { ReactNode, HTMLAttributes } from "react";

type DivProps = HTMLAttributes<HTMLDivElement>;

const Root = ({ children, ...rest }: DivProps): React.JSX.Element => {
  return <div className={`app-shell ${rest.className || ""}`} {...rest}>{children}</div>;
};

const Header = ({ children, ...rest }: HTMLAttributes<HTMLElement>): React.JSX.Element => {
  return <header className={`app-shell-header ${rest.className || ""}`} {...rest}>{children}</header>;
};

const Main = ({ children, ...rest }: HTMLAttributes<HTMLElement>): React.JSX.Element => {
  return <main className={`app-shell-main ${rest.className || ""}`} {...rest}>{children}</main>;
};

export const AppShell = { Root, Header, Main };
