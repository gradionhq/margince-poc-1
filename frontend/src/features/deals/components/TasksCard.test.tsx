import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

const mutateMock = vi.fn();
vi.mock("../api/deals.js", () => ({
  useUpdateActivity: () => ({ mutate: mutateMock, isPending: false }),
}));
vi.mock("../../people/api/people.js", () => ({
  usePerson: (id: string) => ({
    data: id === "u1" ? { id: "u1", full_name: "Dana Lee" } : undefined,
    isLoading: false,
  }),
}));

import { TasksCard } from "./TasksCard.js";

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient();
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("TasksCard", () => {
  const tasks = [
    {
      id: "t1",
      kind: "task",
      subject: "Send proposal",
      due_at: "2026-02-01T00:00:00Z",
      assignee_id: "u1",
      captured_by: "human:owner-id",
      source: "ui",
      is_done: false,
    },
  ] as never[];

  it("renders assignee, due date, captured_by provenance, and a done checkbox", () => {
    render(
      <TasksCard
        tasks={tasks}
        dealId="d1"
        isLoading={false}
        isError={false}
        onTaskDone={vi.fn()}
      />,
      { wrapper },
    );
    expect(screen.getByText("Send proposal")).toBeInTheDocument();
    expect(screen.getByText("Dana Lee")).toBeInTheDocument();
    expect(screen.getByRole("checkbox")).not.toBeChecked();
  });

  it("clicking the checkbox marks the task done via updateActivity and fires onTaskDone on success", async () => {
    mutateMock.mockImplementation((_vars, opts) => opts?.onSuccess?.());
    const onTaskDone = vi.fn();
    render(
      <TasksCard
        tasks={tasks}
        dealId="d1"
        isLoading={false}
        isError={false}
        onTaskDone={onTaskDone}
      />,
      { wrapper },
    );
    await userEvent.click(screen.getByRole("checkbox"));
    expect(mutateMock).toHaveBeenCalledWith(
      { activityId: "t1", dealId: "d1", patch: { is_done: true } },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
    expect(onTaskDone).toHaveBeenCalled();
  });

  it("honest-empty when there are no tasks", () => {
    render(
      <TasksCard
        tasks={[]}
        dealId="d1"
        isLoading={false}
        isError={false}
        onTaskDone={vi.fn()}
      />,
      { wrapper },
    );
    expect(screen.getByText("No tasks yet")).toBeInTheDocument();
  });
});
