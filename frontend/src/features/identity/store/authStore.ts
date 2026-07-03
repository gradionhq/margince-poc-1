import { create } from "zustand";
import type { components } from "../../../lib/api-client/generated/index.js";
import { fetchMe } from "../api/auth.js";

type User = components["schemas"]["User"];

interface AuthState {
  user: User | null;
  role: string | null;
  roles: string[];
  loading: boolean;
}

interface AuthStore extends AuthState {
  _set: (state: Partial<AuthState>) => void;
}

const useAuthStoreInternal = create<AuthStore>((set) => ({
  user: null,
  role: null,
  roles: [],
  loading: true,
  _set: (state) => set(state),
}));

export const useAuthStore = () => {
  const { user, role, roles, loading } = useAuthStoreInternal();
  return { user, role, roles, loading };
};

export async function initAuth(): Promise<void> {
  const result = await fetchMe();
  if (result) {
    useAuthStoreInternal.getState()._set({
      user: result.user,
      role: result.role,
      roles: result.roles,
      loading: false,
    });
  } else {
    useAuthStoreInternal.getState()._set({
      user: null,
      role: null,
      roles: [],
      loading: false,
    });
  }
}

export function setAuth(user: User, role: string, roles: string[] = []): void {
  useAuthStoreInternal.getState()._set({ user, role, roles, loading: false });
}

export function clearAuth(): void {
  useAuthStoreInternal
    .getState()
    ._set({ user: null, role: null, roles: [], loading: false });
}

/** Full reset to initial state — use only in tests to restore loading=true. */
export function resetAuth(): void {
  useAuthStoreInternal
    .getState()
    ._set({ user: null, role: null, roles: [], loading: true });
}
