import { createContext, useContext, type PropsWithChildren } from "react";
import ptBR from "antd/locale/pt_BR";
import { ptBRV1 } from "./v1/pt-BR";

export const I18N_CATALOG_VERSION = 1 as const;
export const DEFAULT_LOCALE = "pt-BR" as const;

const catalogs = {
  "pt-BR": ptBRV1,
} as const;

export type AppLocale = keyof typeof catalogs;
export type AppMessages = (typeof catalogs)[AppLocale];

type I18nContextValue = {
  version: typeof I18N_CATALOG_VERSION;
  locale: AppLocale;
  messages: AppMessages;
};

const defaultContextValue: I18nContextValue = {
  version: I18N_CATALOG_VERSION,
  locale: DEFAULT_LOCALE,
  messages: catalogs[DEFAULT_LOCALE],
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
  return (
    <I18nContext.Provider
      value={{ version: I18N_CATALOG_VERSION, locale, messages: catalogs[locale] }}
    >
      {children}
    </I18nContext.Provider>
  );
}

export function useI18n(): I18nContextValue {
  return useContext(I18nContext) ?? defaultContextValue;
}
