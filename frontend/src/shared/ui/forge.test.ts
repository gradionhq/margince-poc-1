import { describe, expect, it } from "vitest";
import * as forge from "./forge.js";

const EXPECTED = [
  "Button",
  "IconButton",
  "Badge",
  "StatusBadge",
  "Avatar",
  "Icon",
  "TextInput",
  "Modal",
  "ConfirmDialog",
  "Tooltip",
  "Toast",
  "SectionHeader",
  "Divider",
  "Skeleton",
  "RadioGroup",
  "FilterDropdown",
  "Kbd",
  "PopoverPortal",
  "StatusDot",
  "StatusEmoji",
  "PresenceIndicator",
] as const;

describe("forge reuse-as-is barrel", () => {
  it("re-exports every canonical reuse-as-is atom", () => {
    for (const name of EXPECTED) {
      expect(forge, `missing re-export: ${name}`).toHaveProperty(name);
      expect((forge as Record<string, unknown>)[name]).toBeDefined();
    }
  });
});
