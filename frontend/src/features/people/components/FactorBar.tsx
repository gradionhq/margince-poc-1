export function FactorBar({ label, value }: { label: string; value: number }) {
  return (
    <div className="flex items-center gap-gf-xs">
      <span className="text-gf-label text-gf-secondary w-20">{label}</span>
      <div className="w-20 h-1 bg-gf-subtle rounded-full overflow-hidden">
        <div
          className="h-full bg-gf-accent rounded-full"
          style={{ width: `${Math.round(value * 100)}%` }}
        />
      </div>
      <span className="text-gf-label text-gf-secondary">
        {Math.round(value * 100)}
      </span>
    </div>
  );
}
