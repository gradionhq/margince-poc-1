import type { Person } from "../../../lib/api-client/generated/index.js";
import { PersonCard } from "./PersonCard.js";

interface PersonListProps {
  people: Pick<Person, "id" | "full_name" | "emails">[];
  isLoading: boolean;
  isError: boolean;
}

export function PersonList({ people, isLoading, isError }: PersonListProps) {
  if (isLoading) {
    return <p className="p-gf-md text-gf-body text-gf-secondary">Loading…</p>;
  }
  if (isError) {
    return (
      <p className="p-gf-md text-gf-body text-gf-status-danger">
        Failed to load people.
      </p>
    );
  }
  return (
    <main className="p-gf-lg max-w-2xl mx-auto">
      <h1 className="text-gf-title font-semibold mb-gf-md">People</h1>
      <ul className="flex flex-col gap-gf-sm">
        {people.map((person) => (
          <PersonCard
            key={person.id}
            name={person.full_name}
            email={person.emails?.[0]?.email}
          />
        ))}
      </ul>
    </main>
  );
}
