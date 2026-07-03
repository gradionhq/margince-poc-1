import createClient from "openapi-fetch";
import type { paths } from "./generated/index.js";

export const apiClient = createClient<paths>({
  baseUrl: "/api",
});
