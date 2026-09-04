import { Input } from "antd";
import type { InputRef } from "antd/es/input";
import { Search } from "lucide-react";
import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent, type Ref } from "react";
import { useQuery } from "@tanstack/react-query";
import { ICON, ICON_STROKE } from "./icons";
import { useDebouncedValue } from "../hooks/useDebouncedValue";
import {
  getSearchCatalog,
  listProfilesLookup,
  suggestSearchValues,
  type SearchCatalogResponse,
} from "../lib/api/client";
import { useI18n } from "../i18n";

export type SearchFieldMode = "local" | "suggest" | "text";

type MenuItem = {
  key: string;
  insert: string;
  label: string;
  description: string;
  id?: string;
};

export function SearchField(props: {
  mode: SearchFieldMode;
  label: string;
  placeholder: string;
  value: string;
  onChange: (value: string) => void;
  onSubmit?: ((value: string) => void) | undefined;
  disabled?: boolean | undefined;
  loading?: boolean | undefined;
  shortcutHint?: string | undefined;
  inputRef?: Ref<HTMLInputElement | null> | undefined;
  grain?: "profiles" | "documents" | "bills" | undefined;
  lookup?: boolean | undefined;
  onPick?: ((id: string, label: string) => void) | undefined;
}) {
  if (props.mode === "local" || props.mode === "text") {
    return (
      <PlainSearchField
        disabled={props.disabled}
        inputRef={props.inputRef}
        label={props.label}
        placeholder={props.placeholder}
        shortcutHint={props.shortcutHint}
        size={props.mode === "text" ? "middle" : "small"}
        value={props.value}
        onChange={props.onChange}
        onSubmit={props.onSubmit}
      />
    );
  }
  return <RemoteSearchField {...props} mode={props.mode} />;
}

function PlainSearchField({
  label,
  placeholder,
  value,
  onChange,
  onSubmit,
  disabled,
  shortcutHint,
  inputRef,
  size,
}: {
  label: string;
  placeholder: string;
  value: string;
  onChange: (value: string) => void;
  onSubmit?: ((value: string) => void) | undefined;
  disabled?: boolean | undefined;
  shortcutHint?: string | undefined;
  inputRef?: Ref<HTMLInputElement | null> | undefined;
  size: "small" | "middle";
}) {
  return (
    <div className={size === "small" ? "search-field search-field--local" : "search-field"}>
      <Input
        allowClear
        aria-label={label}
        autoComplete="off"
        className="search-field__input"
        disabled={Boolean(disabled)}
        inputMode="search"
        placeholder={placeholder}
        prefix={<Search aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />}
        ref={(node: InputRef | null) => assignInputRef(inputRef, node)}
        size={size}
        spellCheck={false}
        type="search"
        value={value}
        {...(shortcutHint ? { suffix: <kbd>{shortcutHint}</kbd> } : {})}
        onBlur={() => onSubmit?.(value)}
        onChange={(event) => {
          const next = event.target.value;
          if (next === value) return;
          onChange(next);
        }}
        onKeyDown={(event: KeyboardEvent<HTMLInputElement>) => {
          if (event.key !== "Enter") return;
          onSubmit?.(value);
        }}
      />
    </div>
  );
}

function RemoteSearchField({
  label,
  placeholder,
  value,
  onChange,
  onSubmit,
  disabled,
  loading,
  shortcutHint,
  inputRef,
  grain,
  lookup,
  onPick,
}: {
  mode: Exclude<SearchFieldMode, "local" | "text">;
  label: string;
  placeholder: string;
  value: string;
  onChange: (value: string) => void;
  onSubmit?: ((value: string) => void) | undefined;
  disabled?: boolean | undefined;
  loading?: boolean | undefined;
  shortcutHint?: string | undefined;
  inputRef?: Ref<HTMLInputElement | null> | undefined;
  grain?: "profiles" | "documents" | "bills" | undefined;
  lookup?: boolean | undefined;
  onPick?: ((id: string, label: string) => void) | undefined;
}) {
  const { messages } = useI18n();
  const listId = useId();
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const committed = useRef(value);

  const token = currentToken(value);
  const slashOpen = !lookup && token.startsWith("/");
  const fieldHint = lookup ? "" : fieldOf(token);
  const catalog = useQuery({
    queryKey: ["search-catalog"],
    queryFn: ({ signal }) => getSearchCatalog(signal),
    enabled: !lookup && (slashOpen || Boolean(fieldHint)),
    staleTime: 60_000,
  });
  const suggestQ = useDebouncedValue(fieldHint ? valueAfterColon(token) : "", 300);
  const suggestions = useQuery({
    queryKey: ["search-suggest", grain, fieldHint, suggestQ],
    queryFn: ({ signal }) =>
      suggestSearchValues(
        { field: fieldHint, q: suggestQ, limit: 50, ...(grain ? { grain } : {}) },
        signal,
      ),
    enabled: !lookup && open && Boolean(fieldHint) && !slashOpen,
    staleTime: 15_000,
  });
  const lookupQ = useDebouncedValue(lookup ? value : "", 300);
  const people = useQuery({
    queryKey: ["profiles", "lookup", lookupQ],
    queryFn: ({ signal }) => listProfilesLookup(lookupQ, signal),
    enabled: Boolean(lookup) && open,
    staleTime: 15_000,
  });

  const items = useMemo(() => {
    if (lookup) {
      return (people.data?.profiles ?? []).map((profile) => ({
        key: profile.id,
        id: profile.id,
        insert: profile.full_name,
        label: profile.full_name,
        description: "",
      }));
    }
    if (slashOpen) {
      return filterSlashItems(catalog.data, token.slice(1), messages.search.slashMenu);
    }
    return (suggestions.data?.suggestions ?? []).map((hit) => ({
      key: hit.value,
      insert: completeToken(value, `${fieldHint}:${hit.value} `),
      label: hit.label,
      description: hit.value,
    }));
  }, [
    catalog.data,
    fieldHint,
    lookup,
    messages.search.slashMenu,
    people.data?.profiles,
    slashOpen,
    suggestions.data,
    token,
    value,
  ]);

  useEffect(() => {
    setActive(0);
  }, [items.length, token]);

  const commit = (next = value) => {
    if (next === committed.current) return;
    committed.current = next;
    onSubmit?.(next);
  };

  const applyItem = (item: MenuItem) => {
    if (lookup) {
      onChange(item.label);
      onPick?.(item.id ?? "", item.label);
      setOpen(false);
      return;
    }
    onChange(item.insert);
    setOpen(false);
    if (!item.insert.endsWith(":")) commit(item.insert);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Escape") {
      setOpen(false);
      return;
    }
    if (open && items.length > 0 && (event.key === "ArrowDown" || event.key === "ArrowUp")) {
      event.preventDefault();
      setActive((current) =>
        event.key === "ArrowDown"
          ? (current + 1) % items.length
          : (current - 1 + items.length) % items.length,
      );
      return;
    }
    if (event.key === "Enter") {
      if (open && items[active]) {
        event.preventDefault();
        applyItem(items[active]);
        return;
      }
      if (!lookup) commit(value);
    }
  };

  const fetching = catalog.isFetching || suggestions.isFetching || people.isFetching || loading;
  const showList = open && (lookup || slashOpen || Boolean(fieldHint));

  return (
    <div
      aria-controls={listId}
      aria-expanded={open}
      aria-label={label}
      className="search-field"
      role="combobox"
    >
      <Input
        allowClear
        aria-autocomplete="list"
        aria-label={label}
        autoComplete="off"
        className="search-field__input"
        disabled={Boolean(disabled)}
        inputMode="search"
        placeholder={placeholder}
        prefix={<Search aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />}
        ref={(node: InputRef | null) => assignInputRef(inputRef, node)}
        size="middle"
        spellCheck={false}
        type="search"
        value={value}
        {...(open && items[active]
          ? { "aria-activedescendant": `${listId}-${items[active].key}` }
          : {})}
        {...(shortcutHint && !open ? { suffix: <kbd>{shortcutHint}</kbd> } : {})}
        onBlur={() => {
          window.setTimeout(() => setOpen(false), 120);
          if (!lookup) commit(value);
        }}
        onChange={(event) => {
          const next = event.target.value;
          if (next === value) return;
          onChange(next);
          setOpen(lookup || next.includes("/") || next.includes(":"));
          if (next === "") {
            if (lookup) onPick?.("", "");
            else commit("");
          }
        }}
        onFocus={() => {
          setOpen(lookup || slashOpen || Boolean(fieldHint));
        }}
        onKeyDown={onKeyDown}
      />
      {showList ? (
        <ul className="search-field__listbox" id={listId} role="listbox">
          {fetching ? <li className="search-field__status">{messages.search.loadingSuggest}</li> : null}
          {items.length === 0 && !fetching ? (
            <li className="search-field__status">
              {lookup ? messages.search.lookupEmpty : messages.search.slashEmpty}
            </li>
          ) : null}
          {items.map((item, index) => (
            <li
              aria-selected={index === active}
              className={index === active ? "search-field__option is-active" : "search-field__option"}
              id={`${listId}-${item.key}`}
              key={item.key}
              role="option"
              onMouseDown={(event) => {
                event.preventDefault();
                applyItem(item);
              }}
            >
              <span className="search-field__option-label">{item.label}</span>
              {item.description ? (
                <span className="search-field__option-desc">{item.description}</span>
              ) : null}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

function assignInputRef(ref: Ref<HTMLInputElement | null> | undefined, node: InputRef | null) {
  const el = node?.input ?? null;
  if (typeof ref === "function") {
    ref(el);
    return;
  }
  if (ref) ref.current = el;
}

function currentToken(value: string): string {
  const parts = value.split(/\s+/);
  return parts.at(-1) ?? "";
}

function fieldOf(token: string): string {
  const cut = token.indexOf(":");
  if (cut <= 0) return "";
  return token.slice(0, cut).replace(/^-/, "").replace(/^\//, "");
}

function valueAfterColon(token: string): string {
  return token.slice(token.indexOf(":") + 1);
}

function completeToken(value: string, replacement: string): string {
  const trimmed = value.replace(/\S+$/, "");
  return `${trimmed}${replacement}`;
}

function filterSlashItems(
  catalog: SearchCatalogResponse | undefined,
  fragment: string,
  copy: { operators: string; fields: string },
): MenuItem[] {
  const folded = fragment.trim().toLowerCase();
  const operators = (catalog?.operators ?? []).map((item) => ({
    key: `op-${item.token}`,
    insert: item.insert,
    label: item.token,
    description: item.description || copy.operators,
  }));
  const fields = (catalog?.fields ?? []).map((field) => ({
    key: field.key,
    insert: `${field.label.split(" · ")[0]?.toLowerCase() ?? field.key}:`,
    label: field.label,
    description: copy.fields,
  }));
  return [...operators, ...fields]
    .filter(
      (item) =>
        !folded ||
        item.label.toLowerCase().includes(folded) ||
        item.insert.toLowerCase().includes(folded),
    )
    .slice(0, 50);
}
