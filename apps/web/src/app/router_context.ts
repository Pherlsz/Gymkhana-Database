import { QueryClient } from "@tanstack/react-query";

export type RouterContext = { queryClient: QueryClient };
export const queryClient = new QueryClient();
