import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  deleteAdminUser,
  grantUserCapability,
  listAdminUsers,
  provisionAdminUser,
  revokeUserCapability,
  updateAdminUserAccess,
  type AdminAccessUpdate,
} from "../api/client";
import type { GrantableCapability } from "./access";

export const adminUsersKey = ["admin", "users"] as const;

export function adminCapabilitiesKey(userId: string) {
  return ["admin", "capabilities", userId] as const;
}

export function useAdminAccess() {
  const queryClient = useQueryClient();
  const users = useQuery({
    queryKey: adminUsersKey,
    queryFn: ({ signal }) => listAdminUsers(signal),
  });

  const updateAccess = useMutation({
    mutationFn: (input: { userId: string } & AdminAccessUpdate) =>
      updateAdminUserAccess(input.userId, {
        role: input.role,
        active: input.active,
        version: input.version,
        display_name: input.display_name,
        email: input.email,
      }),
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: adminUsersKey });
    },
  });

  const provision = useMutation({
    mutationFn: provisionAdminUser,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: adminUsersKey });
    },
  });

  const removeUser = useMutation({
    mutationFn: deleteAdminUser,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: adminUsersKey });
    },
  });

  const toggleCapability = useMutation({
    mutationFn: (input: { userId: string; capability: GrantableCapability; granted: boolean }) =>
      input.granted
        ? revokeUserCapability(input.userId, input.capability)
        : grantUserCapability(input.userId, input.capability),
    onSettled: (_data, _error, input) => {
      void queryClient.invalidateQueries({ queryKey: adminCapabilitiesKey(input.userId) });
    },
  });

  return { users, updateAccess, provision, removeUser, toggleCapability };
}
