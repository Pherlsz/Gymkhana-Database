import { Button, Flex, Typography } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { SearchField } from "../../components/SearchField";
import { useI18n } from "../../i18n";
import { getProfile } from "../api/client";
import { queryKeys } from "../api/queryKeys";

export function CadastroOwnerPicker({
  ownerId,
  onSelect,
  onClear,
}: {
  ownerId?: string | undefined;
  onSelect: (ownerProfileId: string) => void;
  onClear: () => void;
}) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;
  const [query, setQuery] = useState("");
  const selected = useQuery({
    queryKey: queryKeys.profiles.detail(ownerId),
    queryFn: ({ signal }) => getProfile(ownerId!, signal),
    enabled: Boolean(ownerId),
  });

  if (ownerId) {
    const name = selected.data?.full_name ?? copy.manualOwnerFromFilter;
    return (
      <div className="cadastro-owner">
        <span className="cadastro-owner__label">{copy.manualSearchOwner}</span>
        <Flex align="center" gap="0.5rem" wrap="wrap">
          <Typography.Text strong>{name}</Typography.Text>
          <Button
            type="text"
            onClick={() => {
              setQuery("");
              onClear();
            }}
          >
            {copy.ownerChange}
          </Button>
        </Flex>
      </div>
    );
  }

  return (
    <div className="cadastro-owner">
      <label className="cadastro-owner__field">
        {copy.manualSearchOwner}
        <SearchField
          label={copy.manualSearchOwnerAria}
          lookup
          mode="suggest"
          placeholder={copy.manualSearchOwnerPlaceholder}
          value={query}
          onChange={setQuery}
          onPick={(id, name) => {
            setQuery(name);
            if (id) onSelect(id);
            else onClear();
          }}
        />
      </label>
    </div>
  );
}
