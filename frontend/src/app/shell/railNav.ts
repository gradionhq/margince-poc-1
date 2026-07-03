export type RailNavItem = {
  id: string;
  label: string;
  icon: string; // PascalCase Lucide name (resolved via RailIcon)
  to: string;
  badgeKey?: "tasks" | "inbox";
  adminOnly?: boolean;
};

// Canonical 9-item IA (AC-shell-1, design/mockups/shell.js order).
export const RAIL_NAV: readonly RailNavItem[] = [
  { id: "home", label: "Home", icon: "Home", to: "/home" },
  { id: "contacts", label: "Contacts", icon: "Users", to: "/people" },
  { id: "companies", label: "Companies", icon: "Building2", to: "/companies" },
  { id: "leads", label: "Leads", icon: "UserPlus", to: "/leads" },
  { id: "deals", label: "Deals", icon: "Target", to: "/deals" },
  {
    id: "tasks",
    label: "Tasks",
    icon: "CheckSquare",
    to: "/tasks",
    badgeKey: "tasks",
  },
  {
    id: "inbox",
    label: "Inbox",
    icon: "Inbox",
    to: "/inbox",
    badgeKey: "inbox",
  },
  { id: "reports", label: "Reports", icon: "BarChart3", to: "/reports" },
  { id: "ask-ai", label: "Ask AI", icon: "Sparkles", to: "/ask-ai" },
  {
    id: "members",
    label: "Members",
    icon: "ShieldCheck",
    to: "/admin/members",
    adminOnly: true,
  },
];

export type RailCounts = { tasks?: number; inbox?: number };
