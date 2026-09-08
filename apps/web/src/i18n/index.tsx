import { createContext, useContext, useMemo, type PropsWithChildren } from "react";
import ptBR from "antd/locale/pt_BR";
import { ptBRV1 } from "./v1/pt-BR";

export const I18N_CATALOG_VERSION = 1 as const;
export const DEFAULT_LOCALE = "pt-BR" as const;

const catalogs = {
  "pt-BR": ptBRV1,
} as const;

export type AppLocale = keyof typeof catalogs;
export type AppMessages = (typeof catalogs)[AppLocale];

export function formatMessage(
  template: string,
  params?: Record<string, string | number | boolean | null | undefined>,
): string {
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (match, key) => {
    const value = params[key];
    return value !== undefined && value !== null ? String(value) : match;
  });
}

export const t = formatMessage;

const pluralRulesCache = new Map<string, Intl.PluralRules>();

export function plural(
  count: number,
  one: string,
  other: string,
  locale: AppLocale = DEFAULT_LOCALE,
): string {
  let pr = pluralRulesCache.get(locale);
  if (!pr) {
    pr = new Intl.PluralRules(locale);
    pluralRulesCache.set(locale, pr);
  }
  return pr.select(count) === "one" ? one : other;
}

type I18nContextValue = {
  version: typeof I18N_CATALOG_VERSION;
  locale: AppLocale;
  messages: AppMessages;
  t: typeof formatMessage;
  plural: typeof plural;
};

const defaultContextValue: I18nContextValue = {
  version: I18N_CATALOG_VERSION,
  locale: DEFAULT_LOCALE,
  messages: catalogs[DEFAULT_LOCALE],
  t: formatMessage,
  plural,
};

const I18nContext = createContext<I18nContextValue | null>(null);

const antdLocales = {
  "pt-BR": ptBR,
};

export function resolveAppLocale(language?: string): AppLocale {
  const normalized = (language ?? "").trim().toLowerCase();
  if (normalized === "pt-br" || normalized === "pt") return "pt-BR";
  return DEFAULT_LOCALE;
}

export function getAntdLocale(locale: AppLocale) {
  return antdLocales[locale];
}

export function I18nProvider({ locale, children }: PropsWithChildren<{ locale: AppLocale }>) {
  const value = useMemo(
    (): I18nContextValue => ({
      version: I18N_CATALOG_VERSION,
      locale,
      messages: catalogs[locale],
      t: formatMessage,
      plural,
    }),
    [locale],
  );
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
  return useContext(I18nContext) ?? defaultContextValue;
}
