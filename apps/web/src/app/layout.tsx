import { Alert, Button, Inline, Page, Stack, StatusBadge, Surface } from "@pherlsz/gymkhana-ui";
import { useQuery } from "@tanstack/react-query";
import { Link, Outlet } from "@tanstack/react-router";
import { getAuthSession, logout } from "../lib/api/auth";
import { queryClient } from "./router_context";
import { SessionContext } from "./session";

export function RootLayout() {
  const session = useQuery({
    queryKey: ["auth-session"],
    queryFn: ({ signal }) => getAuthSession(signal),
    retry: false,
  });
  if (session.isLoading) {
    return (
      <Page.Root maxWidth="lg">
        <Page.Content>
          <Alert title="Verificando acesso">Validando sua sessão da aplicação.</Alert>
        </Page.Content>
      </Page.Root>
    );
  }
  if (session.isError || !session.data?.authenticated || !session.data.user) {
    return <LoginScreen />;
  }
  return (
    <SessionContext.Provider value={session.data}>
      <Page.Root maxWidth="xl">
        <Page.Header>
          <Page.Eyebrow>Gymkhana Database</Page.Eyebrow>
          <Page.Title>Base de dados da gincana</Page.Title>
          <Page.Description>
            Cadastros, consultas e operações na nova arquitetura Profile-first.
          </Page.Description>
          <Page.Actions>
            <Inline align="center">
              <StatusBadge tone="success">{session.data.user.role}</StatusBadge>
              <span>{session.data.user.display_name}</span>
              <span>{session.data.user.email}</span>
              <Button
                onClick={() =>
                  void logout().then(() =>
                    queryClient.invalidateQueries({ queryKey: ["auth-session"] }),
                  )
                }
              >
                Sair
              </Button>
            </Inline>
          </Page.Actions>
        </Page.Header>
        <Page.Content>
          <Stack gap="5">
            <nav aria-label="Navegação principal" className="application-nav">
              <Link to="/">Início</Link>
              <Link to="/profiles">Pessoas</Link>
              <Link to="/search">Busca</Link>
              <Link to="/custom-data">Dados personalizados</Link>
              <Link to="/attachments">Anexos</Link>
              <Link to="/operations">Importações e exportações</Link>
              <Link to="/google-forms">Google Forms</Link>
              <Link to="/query">Consultas</Link>
              <Link to="/matching">Duplicatas</Link>
              <Link to="/chat">AI Chat</Link>
              <Link to="/ocr">OCR</Link>
              <Link to="/tasks">Tarefas</Link>
            </nav>
            <Outlet />
          </Stack>
        </Page.Content>
      </Page.Root>
    </SessionContext.Provider>
  );
}

function LoginScreen() {
  return (
    <Page.Root maxWidth="sm">
      <Page.Header>
        <Page.Eyebrow>Acesso restrito</Page.Eyebrow>
        <Page.Title>Entre com Google</Page.Title>
        <Page.Description>
          Somente contas Google previamente autorizadas por e-mail podem acessar esta aplicação.
        </Page.Description>
      </Page.Header>
      <Page.Content>
        <Surface tone="raised">
          <Stack gap="4">
            <p>
              O login usa OpenID Connect, sessão de 24 horas e validação pela lista de e-mails
              permitidos.
            </p>
            <Button onClick={() => window.location.assign("/auth/login")}>Continuar com Google</Button>
          </Stack>
        </Surface>
      </Page.Content>
    </Page.Root>
  );
}
