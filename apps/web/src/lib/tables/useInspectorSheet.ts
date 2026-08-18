import { useMediaQuery } from "../useMediaQuery";
import { DEFAULT_SHEET_PREFERENCES } from "./sheetPreferences";

export function useInspectorSheet(mediaQuery = DEFAULT_SHEET_PREFERENCES.inspectorMediaQuery) {
  return useMediaQuery(mediaQuery);
}
