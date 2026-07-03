import { Icon, TextInput } from "@shared/ui";

export function SearchField({
  value,
  onChange,
  placeholder,
  onClear,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  onClear?: () => void;
}) {
  return (
    <div className="relative flex items-center">
      <TextInput
        value={value}
        onChange={onChange}
        placeholder={placeholder}
        leadingIcon="Search"
        className="w-full"
      />
      {value && (
        <button
          type="button"
          aria-label="Clear search"
          className="absolute right-2 flex items-center justify-center text-gf-secondary"
          onClick={() => {
            onChange("");
            onClear?.();
          }}
        >
          <Icon name="X" size={16} />
        </button>
      )}
    </div>
  );
}
