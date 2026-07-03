import { type FormEvent, useState } from "react";

interface LoginFormProps {
  onSubmit: (values: { email: string; password: string }) => void;
  isSubmitting?: boolean;
  error?: string;
}

export function LoginForm({ onSubmit, isSubmitting, error }: LoginFormProps) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [emailError, setEmailError] = useState("");

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!email) {
      setEmailError("Email is required");
      return;
    }
    setEmailError("");
    onSubmit({ email, password });
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-gf-md">
      <div className="flex flex-col gap-gf-xs">
        <label
          htmlFor="email"
          className="text-gf-caption font-medium text-gf-primary"
        >
          Email
        </label>
        <input
          id="email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          className="border border-gf-subtle rounded-md p-gf-sm text-gf-body text-gf-primary"
          autoComplete="email"
        />
        {emailError && (
          <p className="text-gf-caption text-gf-status-danger">{emailError}</p>
        )}
      </div>
      <div className="flex flex-col gap-gf-xs">
        <label
          htmlFor="password"
          className="text-gf-caption font-medium text-gf-primary"
        >
          Password
        </label>
        <input
          id="password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className="border border-gf-subtle rounded-md p-gf-sm text-gf-body text-gf-primary"
          autoComplete="current-password"
        />
      </div>
      {error && (
        <p className="text-gf-caption text-gf-status-danger">{error}</p>
      )}
      <button
        type="submit"
        disabled={isSubmitting}
        className="bg-gf-accent text-gf-on-accent rounded-md p-gf-sm font-medium"
      >
        {isSubmitting ? "Signing in…" : "Sign in"}
      </button>
    </form>
  );
}
