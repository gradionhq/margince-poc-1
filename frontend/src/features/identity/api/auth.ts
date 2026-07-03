import { apiClient } from "../../../lib/api-client/client.js";
import type { components } from "../../../lib/api-client/generated/index.js";

type User = components["schemas"]["User"];

export async function fetchMe(): Promise<{
  user: User;
  role: string;
  roles: string[];
} | null> {
  const { data, error, response } = await apiClient.GET("/me");
  if (response.status === 401 || error) return null;
  if (!data) return null;
  const roles = data.roles ?? [];
  const role = roles[0] ?? "read_only";
  return { user: data.user, role, roles };
}

export async function login(email: string, password: string): Promise<void> {
  const { response } = await apiClient.POST("/auth/login", {
    body: { email, password },
  });
  if (response.status === 401) {
    throw new Error("Invalid email or password");
  }
  if (!response.ok) {
    throw new Error("Login failed");
  }
}

export async function logout(): Promise<void> {
  await apiClient.POST("/auth/logout", {});
}
