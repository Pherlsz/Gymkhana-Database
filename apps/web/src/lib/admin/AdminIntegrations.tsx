import { Button, Popconfirm, Table, Tag, Typography } from "antd";
import type { TableProps } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getRouteApi, Link, useNavigate } from "@tanstack/react-router";
import { AssistantKeyPanel } from "./AssistantKeyPanel";
import { ADMIN_INTEGRATIONS_RETURN_PATH } from "./adminSearch";
import { QueryView } from "../../components/QueryView";
import { StatusBanner } from "../../components/StatusBanner";
import { useI18n } from "../../i18n";
import { formatDateTime } from "../formatters";
import { queryKeys } from "../api/queryKeys";
import {
  beginGoogleFormsOAuth,
  disconnectGoogleForms,
  getGoogleFormsStatus,
  listGoogleFormsSources,
  type GoogleFormsSource,
} from "../api/googleForms";

const adminRoute = getRouteApi("/admin");

export function AdminIntegrations() {
  const { messages } = useI18n();
  const forms = messages.googleForms;
  const copy = messages.admin.integrations;
  const entities = messages.common.entities;
  const search = adminRoute.useSearch();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const status = useQuery({
    queryKey: queryKeys.googleForms.status,
    queryFn: ({ signal }) => getGoogleFormsStatus(signal),
  });
  const connected = status.data?.connected === true;
  const sources = useQuery({
    queryKey: queryKeys.googleForms.sources,
    queryFn: ({ signal }) => listGoogleFormsSources(signal),
    enabled: connected,
  });

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.googleForms.status });
    void queryClient.invalidateQueries({ queryKey: queryKeys.googleForms.sources });
  };

  const connect = useMutation({
    mutationFn: () => beginGoogleFormsOAuth(ADMIN_INTEGRATIONS_RETURN_PATH),
  });
  const disconnect = useMutation({
    mutationFn: () => {
      const version = status.data?.connection?.version;
      if (version === undefined) throw new Error(forms.disconnectError);
      return disconnectGoogleForms(version);
    },
    onSuccess: refresh,
  });

  const clearOAuth = () => {
    void navigate({ to: "/admin", search: { tab: "integrations" } });
  };

  const columns: TableProps<GoogleFormsSource>["columns"] = [
    {
      title: messages.common.labels.name,
      dataIndex: "title",
      render: (value: string) => (
        <span className="admin-user-cell" title={value}>
          {value}
        </span>
      ),
    },
    {
      title: messages.common.labels.module,
      dataIndex: "module",
      width: 140,
      render: (module: GoogleFormsSource["module"]) => moduleLabel(module, entities),
    },
    {
      title: messages.common.labels.stage,
      dataIndex: "state",
      width: 140,
      render: (state: string) => <Tag>{stateLabel(forms.states, state)}</Tag>,
    },
    {
      key: "open",
      width: 160,
      render: (_value, source) => (
        <Link
          search={{ mode: "forms", source: source.id, tab: "sources", table: "people" }}
          to="/cadastro"
        >
          {copy.openCadastro}
        </Link>
      ),
    },
  ];

  return (
    <div className="admin-integrations">
      {search.google_forms === "connected" ? (
        <StatusBanner closable title={forms.connectedMessage} tone="success" onClose={clearOAuth} />
      ) : null}
      {search.google_forms === "denied" ? (
        <StatusBanner closable title={forms.deniedMessage} tone="warning" onClose={clearOAuth} />
      ) : null}
      <section className="admin-form">
        <header className="admin-form__header">
          <h2 className="admin-form__title">{copy.googleForms}</h2>
          {status.data ? (
            <Tag
              color={
                !status.data.enabled ? "default" : status.data.connected ? "success" : "warning"
              }
            >
              {!status.data.enabled
                ? copy.disabledTag
                : status.data.connected
                  ? copy.connectedTag
                  : copy.disconnectedTag}
            </Tag>
          ) : null}
        </header>
        <QueryView
          compact
          error={status.error}
          errorTitle={forms.statusErrorMessage}
          isError={status.isError}
          isPending={status.isPending}
          onRetry={() => void status.refetch()}
        >
          {status.data && !status.data.enabled ? (
            <p className="admin-form__status">{copy.disabled}</p>
          ) : null}
          {status.data?.enabled && !status.data.connected ? (
            <>
              <p className="admin-form__status">{copy.redirectLabel}</p>
              <Typography.Text className="admin-form__value" copyable>
                {googleFormsRedirectURL()}
              </Typography.Text>
              {connect.isError ? (
                <StatusBanner error={connect.error} title={forms.connectAuthError} />
              ) : null}
              <div className="admin-form__actions">
                <Button loading={connect.isPending} type="primary" onClick={() => connect.mutate()}>
                  {forms.connectButton}
                </Button>
              </div>
            </>
          ) : null}
          {status.data?.connected && status.data.connection ? (
            <div className="admin-form__line">
              <p className="admin-form__status">
                {formatDateTime(status.data.connection.updated_at)} ·{" "}
                {stateLabel(forms.states, status.data.connection.state)}
              </p>
              <Popconfirm
                cancelText={messages.common.actions.cancel}
                okButtonProps={{ danger: true }}
                okText={forms.disconnectButton}
                title={forms.disconnectConfirm}
                onConfirm={() => disconnect.mutate()}
              >
                <Button danger loading={disconnect.isPending}>
                  {forms.disconnectButton}
                </Button>
              </Popconfirm>
            </div>
          ) : null}
          {disconnect.isError ? (
            <StatusBanner error={disconnect.error} title={forms.disconnectError} />
          ) : null}
          {connected ? (
            <QueryView
              compact
              error={sources.error}
              errorTitle={forms.loadSourcesError}
              isError={sources.isError}
              isPending={sources.isPending}
              onRetry={() => void sources.refetch()}
            >
              {sources.isSuccess ? (
                <Table<GoogleFormsSource>
                  columns={columns}
                  dataSource={sources.data.sources}
                  locale={{ emptyText: forms.emptySourcesTitle }}
                  pagination={false}
                  rowKey="id"
                  scroll={{ x: true }}
                />
              ) : null}
            </QueryView>
          ) : null}
        </QueryView>
      </section>
      <AssistantKeyPanel />
    </div>
  );
}

function moduleLabel(
  module: GoogleFormsSource["module"],
  entities: { people: string; documents: string; bills: string },
) {
  if (module === "DOCUMENTS") return entities.documents;
  if (module === "BILLS") return entities.bills;
  return entities.people;
}

function stateLabel(states: { readonly [key: string]: string }, value: string) {
  return states[value] ?? value;
}

function googleFormsRedirectURL(): string {
  const base = String(import.meta.env.VITE_API_BASE_URL || "http://localhost:8080").replace(
    /\/$/,
    "",
  );
  return `${base}/api/v1/google-forms/oauth/callback`;
}
