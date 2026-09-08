import { Spin, Select } from "antd";
import { Plus, Search } from "lucide-react";
import { type ReactNode, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { t } from "../../../i18n";
import { type Profile, listProfilesLookup } from "../../api/client";
import { queryKeys } from "../../api/queryKeys";

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

function scoreProfile(profile: Profile, query: string): number {
  const q = query.trim().toLowerCase();
  if (!q) return 0;

  const cleanDigits = q.replace(/\D/g, "");
  const profileCpfDigits = (profile.cpf ?? "").replace(/\D/g, "");
  if (cleanDigits.length >= 3 && profileCpfDigits) {
    if (profileCpfDigits === cleanDigits) return -2;
    if (profileCpfDigits.startsWith(cleanDigits)) return -1;
    if (profileCpfDigits.includes(cleanDigits)) return 0;
  }

  const normName = profile.full_name.toLowerCase().trim();
  if (normName === q) return 1;
  if (normName.startsWith(`${q} `)) return 2;
  if (normName.startsWith(q)) return 3;

  const words = normName.split(/\s+/);
  if (words.some((w) => w === q)) return 4;
  if (words.some((w) => w.startsWith(q))) return 5;
  if (normName.includes(q)) return 6;
  return 7;
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
    queryKey: queryKeys.profiles.lookup(searchQuery),
    queryFn: ({ signal }) => listProfilesLookup(searchQuery, signal),
    staleTime: 30_000,
  });

  const options = useMemo(() => {
    const list: Array<{ value: string; label: ReactNode; profile?: Profile }> = [];
    const profiles = profilesLookup.data?.profiles ?? [];

    const sortedProfiles = [...profiles].sort((a, b) => {
      const scoreA = scoreProfile(a, searchQuery);
      const scoreB = scoreProfile(b, searchQuery);
      if (scoreA !== scoreB) return scoreA - scoreB;
      return a.full_name.localeCompare(b.full_name, "pt-BR");
    });

    for (const p of sortedProfiles) {
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
              <span>{t(createNewOptionText, { name: trimmed })}</span>
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
      suffixIcon={<Search size={15} style={{ opacity: 0.55 }} />}
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
