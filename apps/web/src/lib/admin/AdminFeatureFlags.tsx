import { Switch, Table, Typography } from "antd";
import type { TableProps } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { QueryView } from "../../components/QueryView";
import { StatusBanner } from "../../components/StatusBanner";
import { useI18n } from "../../i18n";
import {
  listFeatureFlags,
  setFeatureFlag,
  type FeatureFlag,
  type FeatureFlagKey,
  type FeatureFlagsResponse,
} from "../api/client";
import { formatDateTime } from "../formatters";
import { queryKeys } from "../api/queryKeys";

function patchFlag(
  previous: FeatureFlagsResponse | undefined,
  next: Pick<FeatureFlag, "key" | "enabled"> & Partial<Pick<FeatureFlag, "updated_at">>,
): FeatureFlagsResponse | undefined {
  if (!previous) return previous;
  return {
    flags: previous.flags.map((flag) =>
      flag.key === next.key ? { ...flag, ...next } : flag,
    ),
  };
}

export function AdminFeatureFlags() {
  const { messages } = useI18n();
  const copy = messages.admin.featureFlags;
  const queryClient = useQueryClient();
  const flags = useQuery({
    queryKey: queryKeys.admin.featureFlags,
    queryFn: ({ signal }) => listFeatureFlags(signal),
  });

  const toggle = useMutation({
    mutationFn: ({ key, enabled }: { key: FeatureFlagKey; enabled: boolean }) =>
      setFeatureFlag(key, enabled),
    onMutate: async ({ key, enabled }) => {
      await queryClient.cancelQueries({ queryKey: queryKeys.admin.featureFlags });
      const previous = queryClient.getQueryData<FeatureFlagsResponse>(queryKeys.admin.featureFlags);
      queryClient.setQueryData(queryKeys.admin.featureFlags, patchFlag(previous, { key, enabled }));
      return { previous };
    },
    onError: (_error, _variables, context) => {
      if (context?.previous) {
        queryClient.setQueryData(queryKeys.admin.featureFlags, context.previous);
      }
    },
    onSuccess: (updated) => {
      queryClient.setQueryData(queryKeys.admin.featureFlags, (previous: FeatureFlagsResponse | undefined) =>
        patchFlag(previous, updated),
      );
    },
  });

  const columns: TableProps<FeatureFlag>["columns"] = [
    {
      title: copy.columns.feature,
      dataIndex: "key",
      render: (key: FeatureFlagKey) => (
        <div>
          <Typography.Text strong>{copy.labels[key]}</Typography.Text>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            {copy.descriptions[key]}
          </Typography.Paragraph>
        </div>
      ),
    },
    {
      title: copy.columns.updated,
      dataIndex: "updated_at",
      width: 200,
      render: (value: string) => formatDateTime(value),
    },
    {
      title: copy.columns.enabled,
      dataIndex: "enabled",
      width: 120,
      render: (enabled: boolean, row) => {
        const busy = toggle.isPending && toggle.variables?.key === row.key;
        return (
          <Switch
            checked={enabled}
            disabled={busy}
            loading={busy}
            onChange={(next) => toggle.mutate({ key: row.key, enabled: next })}
          />
        );
      },
    },
  ];

  return (
    <section className="admin-panel">
      <Typography.Title level={4}>{copy.title}</Typography.Title>
      <Typography.Paragraph type="secondary">{copy.description}</Typography.Paragraph>
      {toggle.isError ? <StatusBanner error={toggle.error} title={copy.saveError} /> : null}
      <QueryView
        error={flags.error}
        errorTitle={copy.loadError}
        isError={flags.isError}
        isPending={flags.isPending}
        onRetry={() => void flags.refetch()}
      >
        <Table
          rowKey="key"
          columns={columns}
          dataSource={flags.data?.flags ?? []}
          pagination={false}
          size="middle"
        />
      </QueryView>
    </section>
  );
}
