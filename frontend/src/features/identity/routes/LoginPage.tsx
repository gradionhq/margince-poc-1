import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { fetchMe, login } from "../api/auth.js";
import { LoginForm } from "../components/LoginForm.js";
import { setAuth } from "../store/authStore.js";

export function LoginPage() {
  const navigate = useNavigate();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | undefined>();

  async function handleSubmit({
    email,
    password,
  }: {
    email: string;
    password: string;
  }) {
    setIsSubmitting(true);
    setError(undefined);
    try {
      await login(email, password);
      const me = await fetchMe();
      if (me) {
        setAuth(me.user, me.role, me.roles);
      }
      navigate("/people", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="flex items-center justify-center min-h-screen bg-gf-page">
      <div className="w-full max-w-sm p-gf-xl bg-gf-card border border-gf-subtle rounded-xl shadow-sm flex flex-col gap-gf-lg">
        <h1 className="text-gf-title font-semibold text-gf-primary text-center">
          Margince
        </h1>
        <LoginForm
          onSubmit={handleSubmit}
          isSubmitting={isSubmitting}
          error={error}
        />
      </div>
    </div>
  );
}
