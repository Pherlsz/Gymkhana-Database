import { createFileRoute } from "@tanstack/react-router";
import { ChatPage, normalizeChatSearch } from "../ChatPage";

export const Route = createFileRoute("/chat")({
  validateSearch: normalizeChatSearch,
  component: ChatPage,
});
