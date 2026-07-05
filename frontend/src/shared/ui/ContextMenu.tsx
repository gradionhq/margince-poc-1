import { PopoverPortal } from "@shared/ui";
import {
  cloneElement,
  isValidElement,
  type KeyboardEvent,
  type KeyboardEventHandler,
  type MouseEventHandler,
  type ReactElement,
  type ReactNode,
  useRef,
  useState,
} from "react";

type TriggerProps = {
  onClick?: MouseEventHandler;
  onKeyDown?: KeyboardEventHandler;
};

type MenuItem = { id: string; label: string; onSelect: () => void };

export function ContextMenu({
  trigger,
  items,
}: {
  trigger: ReactNode;
  items: ReadonlyArray<MenuItem>;
}) {
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const anchorRef = useRef<HTMLDivElement>(null);

  function toggle() {
    setOpen((o) => !o);
    setActiveIndex(-1);
  }

  const triggerElement = isValidElement(trigger)
    ? (trigger as ReactElement<TriggerProps>)
    : null;
  const triggerWithOpen = triggerElement
    ? cloneElement(triggerElement, {
        onClick: (event) => {
          triggerElement.props.onClick?.(event);
          toggle();
        },
      })
    : trigger;

  function handleKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((i) => (i + 1) % items.length);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((i) => (i - 1 + items.length) % items.length);
    } else if (e.key === "Enter") {
      const item = items[activeIndex];
      if (item) {
        item.onSelect();
        setOpen(false);
      }
    } else if (e.key === "Escape") {
      setOpen(false);
    }
  }

  return (
    <div ref={anchorRef} className="inline-flex">
      {triggerWithOpen}
      {open && (
        <PopoverPortal
          anchorRef={anchorRef}
          placement="bottom-left"
          onClickOutside={() => setOpen(false)}
        >
          <div
            role="menu"
            tabIndex={0}
            onKeyDown={handleKeyDown}
            // min-width uses the forge component token (no Tailwind utility is
            // bound to it yet — reach via var() per the token catalog).
            style={{ minWidth: "var(--gf-component-context-menu-min-w)" }}
            className="flex flex-col rounded-md bg-gf-elevated p-gf-xs shadow-sm"
          >
            {items.map((item, i) => (
              <div
                key={item.id}
                role="menuitem"
                tabIndex={-1}
                className={`cursor-pointer rounded-md px-gf-sm py-gf-xs text-gf-body text-gf-primary ${i === activeIndex ? "bg-gf-hover" : ""}`}
                onClick={() => {
                  item.onSelect();
                  setOpen(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    item.onSelect();
                    setOpen(false);
                  }
                }}
              >
                {item.label}
              </div>
            ))}
          </div>
        </PopoverPortal>
      )}
    </div>
  );
}
