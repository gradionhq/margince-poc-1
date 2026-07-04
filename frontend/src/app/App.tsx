import { Navigate, Route, Routes } from "react-router-dom";
import { LoginPage } from "../features/identity/routes/LoginPage.js";
import { CompaniesPage } from "../features/organizations/routes/CompaniesPage.js";
import { CompanyDetailPage } from "../features/organizations/routes/CompanyDetailPage.js";
import { PeoplePage } from "../features/people/routes/PeoplePage.js";
import { ProtectedRoute } from "./ProtectedRoute.js";
import { ShellPlaceholderPage } from "./ShellPlaceholderPage.js";
import { AppShell } from "./shell/AppShell.js";

export default function App() {
  return (
    <Routes>
      {/* Rail-less exceptions: login (+ future client-surfaces / onboarding index). */}
      <Route path="/login" element={<LoginPage />} />

      {/* Protected app shell (rail + top bar) wrapping the product screens. */}
      <Route
        element={
          <ProtectedRoute>
            <AppShell />
          </ProtectedRoute>
        }
      >
        <Route path="/home" element={<ShellPlaceholderPage title="Home" />} />
        <Route path="/people" element={<PeoplePage />} />
        <Route path="/companies" element={<CompaniesPage />} />
        <Route path="/companies/:id" element={<CompanyDetailPage />} />
        <Route path="/leads" element={<ShellPlaceholderPage title="Leads" />} />
        <Route path="/deals" element={<ShellPlaceholderPage title="Deals" />} />
        <Route path="/tasks" element={<ShellPlaceholderPage title="Tasks" />} />
        <Route path="/inbox" element={<ShellPlaceholderPage title="Inbox" />} />
        <Route
          path="/reports"
          element={<ShellPlaceholderPage title="Reports" />}
        />
        <Route
          path="/ask-ai"
          element={<ShellPlaceholderPage title="Ask AI" />}
        />
        <Route
          path="/settings"
          element={<ShellPlaceholderPage title="Settings" />}
        />
        <Route
          path="/admin/members"
          element={<ShellPlaceholderPage title="Members" />}
        />
      </Route>

      <Route path="/" element={<Navigate to="/home" replace />} />
      <Route path="*" element={<Navigate to="/home" replace />} />
    </Routes>
  );
}
