import { createFileRoute } from "@tanstack/react-router";
import { OCRPage, normalizeOCRSearch } from "../OCRPage";

export const Route = createFileRoute("/ocr")({
  validateSearch: normalizeOCRSearch,
  component: OCRPage,
});
