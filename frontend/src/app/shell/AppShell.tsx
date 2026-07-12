import { useState } from "react";
import { Outlet } from "react-router-dom";
import { useAuthStore } from "../../features/identity/store/authStore.js";
import { ToastContainer } from "../../shared/ui/ToastContainer.js";
import { useActiveNavId } from "../../shared/ui/useActiveNavId.js";
import { RAIL_NAV, type RailCounts } from "./railNav.js";
import { TopBar } from "./TopBar.js";
import { WorkspaceRail } from "./WorkspaceRail.js";

type ToastState = { id: string; message: string; variant?: string };

// No feature currently pushes toasts; this mirrors the local useToasts()
// shape used by the (now-pruned) pages so ToastContainer has a real,
// app-wide mount point ready for the next feature that needs it.
function useToasts() {
  const [toasts, setToasts] = useState<ToastState[]>([]);
  const dismiss = (id: string) =>
    setToasts((t) => t.filter((x) => x.id !== id));
  return { toasts, dismiss };
}

export function AppShell({ counts }: { counts?: RailCounts } = {}) {
  const activeId = useActiveNavId();
  const title = RAIL_NAV.find((i) => i.id === activeId)?.label ?? "";
  const { user, roles } = useAuthStore();
  const isAdmin = roles.includes("admin");
  const { toasts, dismiss } = useToasts();
  return (
    <div className="flex h-screen w-screen overflow-hidden bg-gf-page">
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-gf-max focus:rounded-md focus:bg-gf-card focus:px-gf-md focus:py-gf-sm focus:text-gf-body focus:text-gf-primary focus:ring-2 focus:ring-gf-accent focus:ring-offset-2 focus:ring-offset-gf-card focus:outline-none"
      >
        Skip to main content
      </a>
      <WorkspaceRail
        activeId={activeId}
        counts={counts}
        isAdmin={isAdmin}
        userName={user?.display_name}
      />
      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar title={title} />
        <main
          id="main-content"
          tabIndex={-1}
          className="min-h-0 flex-1 overflow-auto"
        >
          <Outlet />
        </main>
      </div>
      <ToastContainer toasts={toasts} onDismiss={dismiss} />
    </div>
  );
}
