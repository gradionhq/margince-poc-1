import type { Activity } from "../../../lib/api-client/generated/index.js";
import { Skeleton } from "../../../shared/ui/forge.js";
import { usePerson } from "../../people/api/people.js";
import { SourceChip } from "../../people/components/SourceChip.js";
import { useUpdateActivity } from "../api/deals.js";

function TaskRow({
  task,
  dealId,
  onToggled,
}: {
  task: Activity;
  dealId: string;
  onToggled: () => void;
}) {
  const { data: assignee } = usePerson(task.assignee_id ?? undefined);
  const update = useUpdateActivity();

  return (
    <li
      data-testid={`task-row-${task.id}`}
      className="flex items-center justify-between gap-gf-sm py-gf-xs border-b border-gf-subtle last:border-b-0"
    >
      <div className="flex items-center gap-gf-sm">
        <input
          type="checkbox"
          checked={task.is_done}
          onChange={() => {
            update.mutate(
              { activityId: task.id, dealId, patch: { is_done: true } },
              { onSuccess: onToggled },
            );
          }}
        />
        <div>
          <p className="text-gf-body text-gf-primary">{task.subject ?? "Task"}</p>
          <p className="text-gf-caption text-gf-secondary">
            <span>{assignee?.full_name ?? "Unassigned"}</span>
            {task.due_at && (
              <span> · due {new Date(task.due_at).toLocaleDateString()}</span>
            )}
          </p>
        </div>
      </div>
      <SourceChip source={task.source} capturedBy={task.captured_by} />
    </li>
  );
}

// `onTaskDone` fires on a successful is_done PATCH — `useUpdateActivity`'s own `onSettled`
// (Task 1) already invalidates `dealsKeys.activities(dealId)` so the list re-renders with
// `is_done: true`; `onTaskDone` exists purely so the parent screen (DealDetailPage, Task 7) can
// show a success toast without this card reaching into forge Toast internals itself.
export function TasksCard({
  tasks,
  dealId,
  isLoading,
  isError,
  onTaskDone,
}: {
  tasks: Activity[];
  dealId: string;
  isLoading: boolean;
  isError: boolean;
  onTaskDone: () => void;
}) {
  return (
    <div data-testid="tasks-card" className="rounded-lg border border-gf-subtle bg-gf-card p-gf-md">
      <h3 className="text-gf-body font-semibold text-gf-primary mb-gf-sm">Tasks</h3>
      {isLoading ? (
        <div data-testid="tasks-card-skeleton">
          <Skeleton height="60px" />
        </div>
      ) : isError ? (
        <p className="text-gf-body text-gf-status-danger">Failed to load tasks.</p>
      ) : tasks.length === 0 ? (
        <p className="text-gf-body text-gf-secondary">No tasks yet</p>
      ) : (
        <ul>
          {tasks.map((t) => (
            <TaskRow key={t.id} task={t} dealId={dealId} onToggled={onTaskDone} />
          ))}
        </ul>
      )}
    </div>
  );
}
