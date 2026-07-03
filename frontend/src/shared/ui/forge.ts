// Reuse-as-is Forge atom vocabulary (EP09 §3). Re-exported straight from
// @shared/ui under their canonical names — NOT re-skinned. Screens import the
// `gw-ui` vocabulary from here so there is one shared component vocabulary.
export {
  Avatar,
  Badge,
  Button,
  ConfirmDialog,
  Divider,
  FilterDropdown,
  Icon,
  IconButton,
  Kbd,
  Modal,
  PopoverPortal,
  PresenceIndicator,
  RadioGroup,
  SectionHeader,
  Skeleton,
  StatusBadge,
  StatusDot,
  StatusEmoji,
  TextInput,
  Toast,
  Tooltip,
} from "@shared/ui";

// gw-ui shell icon entry point: Forge `Icon` for registered names, with a
// lucide-react fallback for shell names Forge does not register (PascalCase
// only — never a raw glyph). See app/shell/RailIcon.tsx (shell-only, not a
// shared/ui atom — re-exported here so app/shell keeps one import site for
// the gw-ui vocabulary).
export { RailIcon } from "../../app/shell/RailIcon.js";
