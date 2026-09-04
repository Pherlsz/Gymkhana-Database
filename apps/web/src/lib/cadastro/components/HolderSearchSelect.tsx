import { Spin, Select } from "antd";
import { Plus } from "lucide-react";
import { type ReactNode, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { type Profile, listProfilesLookup } from "../../api/client";

export interface HolderSearchSelectProps {
  id?: string;
  ariaLabel?: string;
  value?: string;
  selectedProfile: Profile | null;
  onSelectProfile: (profile: Profile) => void;
  onSelectNewName: (name: string) => void;
  onClear: () => void;
  placeholder: string;
  createNewOptionText: string;
  disabled?: boolean;
}

export function HolderSearchSelect({
  id = "holder-search-select",
  ariaLabel,
  value,
  selectedProfile,
  onSelectProfile,
  onSelectNewName,
  onClear,
  placeholder,
  createNewOptionText,
  disabled,
}: HolderSearchSelectProps) {
  const [searchQuery, setSearchQuery] = useState("");

  const profilesLookup = useQuery({
    queryKey: ["profiles-lookup", searchQuery],
    queryFn: ({ signal }) => listProfilesLookup(searchQuery, signal),
    staleTime: 30_000,
  });

  const options = useMemo(() => {
    const list: Array<{ value: string; label: ReactNode; profile?: Profile }> = [];
    const profiles = profilesLookup.data?.profiles ?? [];

    for (const p of profiles) {
      list.push({
        value: p.id,
        label: (
          <div className="profile-select-option">
            <span className="profile-select-option__name">{p.full_name}</span>
            <span className="profile-select-option__meta">
              {p.cpf ? `CPF: ${p.cpf}` : ""}
              {p.cpf && (p.email || p.mobile_phone) ? " · " : ""}
              {p.mobile_phone || p.email || ""}
            </span>
          </div>
        ),
        profile: p,
      });
    }

    const trimmed = searchQuery.trim();
    if (trimmed.length > 0) {
      const hasExactMatch = profiles.some(
        (p) => p.full_name.toLowerCase() === trimmed.toLowerCase(),
      );
      if (!hasExactMatch) {
        list.push({
          value: `__new__:${trimmed}`,
          label: (
            <div className="profile-select-option profile-select-option--new">
              <span className="profile-select-option__new-badge">
                <Plus size={12} strokeWidth={2.5} />
              </span>
              <span>{createNewOptionText.replace("{name}", trimmed)}</span>
            </div>
          ),
        });
      }
    }

    return list;
  }, [profilesLookup.data?.profiles, searchQuery, createNewOptionText]);

  const handleSelect = (val?: string) => {
    if (!val) {
      onClear();
      return;
    }
    if (val.startsWith("__new__:")) {
      const typedName = val.replace("__new__:", "").trim();
      onSelectNewName(typedName);
      setSearchQuery("");
      return;
    }
    const found = profilesLookup.data?.profiles?.find((p) => p.id === val);
    if (found) {
      onSelectProfile(found);
      setSearchQuery("");
    }
  };

  return (
    <Select
      allowClear
      aria-label={ariaLabel ?? placeholder}
      disabled={Boolean(disabled)}
      filterOption={false}
      id={id}
      notFoundContent={profilesLookup.isLoading ? <Spin size="small" /> : null}
      options={options}
      placeholder={placeholder}
      showSearch
      style={{ width: "100%" }}
      value={selectedProfile ? selectedProfile.id : value || undefined}
      onChange={(val) => {
        if (!val) onClear();
      }}
      onSearch={(query) => {
        setSearchQuery(query);
        if (!selectedProfile) {
          onSelectNewName(query);
        }
      }}
      onSelect={handleSelect}
    />
  );
}
