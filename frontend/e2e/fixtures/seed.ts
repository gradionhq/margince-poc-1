import { type APIRequestContext, expect } from "@playwright/test";

const API_URL = process.env.E2E_API_URL ?? "http://localhost:8080";

type Person = {
  id: string;
  full_name: string;
  archived_at?: string | null;
};

type Organization = {
  id: string;
  display_name: string;
  archived_at?: string | null;
};

type Deal = {
  id: string;
  name: string;
  archived_at?: string | null;
};

function uniqueSuffix(prefix: string): string {
  return `${prefix}-${Date.now().toString(36)}-${Math.random()
    .toString(36)
    .slice(2, 8)}`;
}

async function readJson<T>(
  response: { ok(): boolean; status(): number; text(): Promise<string> },
  label: string,
): Promise<T> {
  const body = await response.text();
  expect(response.ok(), `${label} failed: ${response.status()} ${body}`).toBe(
    true,
  );
  return JSON.parse(body) as T;
}

async function createPerson(
  request: APIRequestContext,
  fullName: string,
  email: string,
): Promise<Person> {
  const response = await request.post(`${API_URL}/people`, {
    data: {
      full_name: fullName,
      emails: [
        {
          email,
          email_type: "work",
          is_primary: true,
          position: 0,
        },
      ],
      source: "manual",
      captured_by: "human:e2e",
    },
  });
  return readJson<Person>(response, `create person ${fullName}`);
}

async function archivePerson(
  request: APIRequestContext,
  id: string,
): Promise<Person> {
  const response = await request.delete(`${API_URL}/people/${id}`);
  return readJson<Person>(response, `archive person ${id}`);
}

async function createOrganization(
  request: APIRequestContext,
  displayName: string,
): Promise<Organization> {
  const suffix = uniqueSuffix("company");
  const response = await request.post(`${API_URL}/organizations`, {
    data: {
      display_name: displayName,
      domains: [{ domain: `${suffix}.example.test`, is_primary: true }],
      source: "manual",
      captured_by: "human:e2e",
    },
  });
  return readJson<Organization>(response, `create organization ${displayName}`);
}

async function archiveOrganization(
  request: APIRequestContext,
  id: string,
): Promise<Organization> {
  const response = await request.delete(`${API_URL}/organizations/${id}`);
  return readJson<Organization>(response, `archive organization ${id}`);
}

async function getDefaultPipelineAndStage(request: APIRequestContext): Promise<{
  pipelineId: string;
  stageId: string;
}> {
  const pipelinesResponse = await request.get(`${API_URL}/pipelines`);
  const pipelines = await readJson<{
    data: Array<{ id: string; is_default: boolean }>;
  }>(pipelinesResponse, "list pipelines");
  const pipeline =
    pipelines.data.find((p) => p.is_default) ?? pipelines.data[0];
  expect(pipeline, "expected a seeded pipeline").toBeTruthy();

  const stagesResponse = await request.get(
    `${API_URL}/stages?${new URLSearchParams({ pipeline_id: pipeline.id }).toString()}`,
  );
  const stages = await readJson<{
    data: Array<{ id: string; semantic: string; position: number }>;
  }>(stagesResponse, "list stages");
  const stage =
    stages.data.find((s) => s.semantic === "open" && s.position === 1) ??
    stages.data.find((s) => s.semantic === "open");
  expect(stage, "expected an open stage").toBeTruthy();
  return { pipelineId: pipeline.id, stageId: stage.id };
}

async function createDeal(
  request: APIRequestContext,
  name: string,
  organizationId?: string,
): Promise<Deal> {
  const { pipelineId, stageId } = await getDefaultPipelineAndStage(request);
  const response = await request.post(`${API_URL}/deals`, {
    data: {
      name,
      pipeline_id: pipelineId,
      stage_id: stageId,
      organization_id: organizationId ?? null,
      amount_minor: 100000,
      currency: "USD",
      source: "manual",
      captured_by: "human:e2e",
    },
  });
  return readJson<Deal>(response, `create deal ${name}`);
}

async function archiveDeal(
  request: APIRequestContext,
  id: string,
): Promise<Deal> {
  const response = await request.delete(`${API_URL}/deals/${id}`);
  return readJson<Deal>(response, `archive deal ${id}`);
}

export async function seedLivePerson(
  request: APIRequestContext,
): Promise<Person> {
  const suffix = uniqueSuffix("person");
  return createPerson(request, `T23 E2E ${suffix}`, `${suffix}@example.test`);
}

export async function seedArchivedPerson(
  request: APIRequestContext,
): Promise<Person> {
  const live = await seedLivePerson(request);
  return archivePerson(request, live.id);
}

export async function seedPersonRestoreConflict(
  request: APIRequestContext,
): Promise<{ archived: Person; live: Person }> {
  const suffix = uniqueSuffix("conflict");
  const email = `${suffix}@example.test`;
  const archived = await createPerson(request, `Archived ${suffix}`, email);
  const archivedRow = await archivePerson(request, archived.id);
  const live = await createPerson(request, `Live ${suffix}`, email);
  return { archived: archivedRow, live };
}

export async function seedLiveOrganization(
  request: APIRequestContext,
): Promise<Organization> {
  const suffix = uniqueSuffix("company");
  return createOrganization(request, `T23 E2E ${suffix}`);
}

export async function seedArchivedOrganization(
  request: APIRequestContext,
): Promise<Organization> {
  const live = await seedLiveOrganization(request);
  return archiveOrganization(request, live.id);
}

export async function seedLiveDeal(request: APIRequestContext): Promise<Deal> {
  const suffix = uniqueSuffix("deal");
  return createDeal(request, `T23 E2E ${suffix}`);
}

export async function seedArchivedDeal(
  request: APIRequestContext,
): Promise<Deal> {
  const live = await seedLiveDeal(request);
  return archiveDeal(request, live.id);
}
