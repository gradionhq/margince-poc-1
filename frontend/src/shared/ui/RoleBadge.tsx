const ROLE_LABELS: Record<string, string> = {
  admin: "Admin",
  manager: "Manager",
  rep: "Rep",
  read_only: "Read Only",
  ops: "Ops",
};

interface RoleBadgeProps {
  role: string;
}

export function RoleBadge({ role }: RoleBadgeProps) {
  const label = ROLE_LABELS[role] ?? role;
  return (
    <span className="inline-flex items-center px-gf-sm py-gf-xs rounded-full text-gf-caption font-medium bg-gf-card border border-gf-subtle text-gf-secondary">
      {label}
    </span>
  );
}
