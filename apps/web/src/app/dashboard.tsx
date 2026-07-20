import { Alert, Inline, Stack, StatusBadge, Surface } from "@pherlsz/gymkhana-ui";
import { useQuery } from "@tanstack/react-query";
import { AdminUsersPanel } from "../AdminUsersPanel";
import type { HealthStatus } from "../generated/api";
import { getHealth } from "../lib/api/client";
import { useApplicationSession } from "./session";

export function Dashboard() {
  const session = useApplicationSession();
  const health = useQuery({ queryKey: ["health"], queryFn: ({ signal }) => getHealth(signal) });
  return (
    <Stack gap="5">
      <Surface tone="raised">
        <Stack gap="3">
          <h2>Ambiente</h2>
          {health.isLoading ? <span>Consultando API...</span> : null}
          {health.isError ? (
            <Alert title="API indisponível" tone="danger">
              Não foi possível consultar o estado da API.
            </Alert>
          ) : null}
          {health.data ? <HealthSummary value={health.data} /> : null}
        </Stack>
      </Surface>
      {session.user.role === "SUPERADMIN" ? (
        <Surface tone="raised">
          <Stack gap="4">
            <div>
              <h2>Usuários da aplicação</h2>
              <p>
                Gerencie funções e acessos. O superadmin único e sua própria conta permanecem
                protegidos.
              </p>
            </div>
            <AdminUsersPanel currentEmail={session.user.email} />
          </Stack>
        </Surface>
      ) : null}
    </Stack>
  );
}

function HealthSummary({ value }: { value: HealthStatus }) {
  return (
    <Inline align="center">
      <StatusBadge tone={value.status === "ok" ? "success" : "warning"}>
        API {value.status}
      </StatusBadge>
      <StatusBadge tone={value.database === "ok" ? "success" : "warning"}>
        Banco {value.database}
      </StatusBadge>
      <span>{value.environment}</span>
      <span>v{value.version}</span>
    </Inline>
  );
}
