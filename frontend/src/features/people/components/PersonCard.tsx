interface PersonCardProps {
  name: string;
  email?: string;
}

export function PersonCard({ name, email }: PersonCardProps) {
  return (
    <li className="flex items-center gap-gf-sm bg-gf-card border border-gf-subtle rounded-md p-gf-md">
      <div className="flex-1 min-w-0">
        <p className="text-gf-body font-medium text-gf-primary truncate">
          {name}
        </p>
        <p className="text-gf-caption text-gf-secondary truncate">
          {email ?? "no email"}
        </p>
      </div>
    </li>
  );
}
