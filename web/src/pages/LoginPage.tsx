// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useNavigate, useLocation, Link } from "react-router-dom";
import { LogIn } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useLocalLogin } from "@/lib/api";
import { ApiError } from "@/lib/queryClient";
import { useBrand } from "@/contexts/BrandContext";

interface FieldErrors {
  email?: string;
  password?: string;
}

function loginErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 401) return "Invalid email or password.";
    if (error.status === 403) return "Your account has been deactivated. Contact an administrator.";
    if (error.status === 429) return "Too many sign-in attempts. Please wait and try again.";
  }
  return "Something went wrong. Please try again.";
}

function BrandHeader() {
  const brand = useBrand();
  return (
    <div className="mb-8 text-center">
      {brand.logo_url ? (
        <img
          src={brand.logo_url}
          alt={brand.app_name}
          className="mx-auto mb-3 h-10 w-auto"
        />
      ) : (
        <h1 className="text-3xl font-bold tracking-tight text-primary">
          {brand.app_name}
        </h1>
      )}
      {brand.company && (
        <p className="mt-1 text-sm text-muted-foreground">{brand.company}</p>
      )}
    </div>
  );
}

// ── Local mode: email + password form ────────────────────────────────────────

function LocalLoginForm({ redirectTo }: { redirectTo: string }) {
  const navigate = useNavigate();
  const login = useLocalLogin();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});

  function validate(): FieldErrors {
    const errs: FieldErrors = {};
    if (!email.trim()) {
      errs.email = "Email is required.";
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      errs.email = "Enter a valid email address.";
    }
    if (!password) errs.password = "Password is required.";
    return errs;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const errs = validate();
    if (Object.keys(errs).length > 0) {
      setFieldErrors(errs);
      return;
    }
    setFieldErrors({});
    try {
      await login.mutateAsync({ email, password });
      navigate(redirectTo, { replace: true });
    } catch {
      // error rendered below via login.error
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4" noValidate>
      <div className="space-y-1.5">
        <Label htmlFor="email">Email</Label>
        <Input
          id="email"
          type="email"
          autoComplete="email"
          placeholder="you@example.com"
          value={email}
          onChange={(e) => {
            setEmail(e.target.value);
            if (fieldErrors.email) setFieldErrors((p) => ({ ...p, email: undefined }));
          }}
          aria-invalid={!!fieldErrors.email}
        />
        {fieldErrors.email && (
          <p className="text-xs text-destructive">{fieldErrors.email}</p>
        )}
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="password">Password</Label>
        <Input
          id="password"
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={(e) => {
            setPassword(e.target.value);
            if (fieldErrors.password) setFieldErrors((p) => ({ ...p, password: undefined }));
          }}
          aria-invalid={!!fieldErrors.password}
        />
        {fieldErrors.password && (
          <p className="text-xs text-destructive">{fieldErrors.password}</p>
        )}
      </div>

      {login.isError && (
        <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {loginErrorMessage(login.error)}
        </p>
      )}

      <Button type="submit" className="w-full" disabled={login.isPending}>
        {login.isPending ? (
          <span className="flex items-center gap-2">
            <span className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
            Signing in…
          </span>
        ) : (
          <span className="flex items-center gap-2">
            <LogIn className="h-4 w-4" />
            Sign in
          </span>
        )}
      </Button>

      <p className="text-center text-xs text-muted-foreground">
        No account yet?{" "}
        <Link to="/register" className="text-primary underline-offset-4 hover:underline">
          Create the first admin account
        </Link>
      </p>
    </form>
  );
}

// ── OIDC mode: SSO redirect button ───────────────────────────────────────────

function OIDCLoginButton() {
  return (
    <div className="space-y-3">
      <p className="text-sm text-muted-foreground text-center">
        Sign in via your organisation's identity provider.
      </p>
      {/* Full page navigation — must exit the SPA so the server handles the redirect */}
      <a href="/auth/login" className="block w-full">
        <Button className="w-full" asChild>
          <span className="flex items-center gap-2">
            <LogIn className="h-4 w-4" />
            Sign in with SSO
          </span>
        </Button>
      </a>
    </div>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────────

export function LoginPage() {
  const location = useLocation();
  const brand = useBrand();
  const from = (location.state as { from?: Location } | null)?.from?.pathname ?? "/";

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm">
        <BrandHeader />
        <Card>
          <CardHeader>
            <CardTitle className="text-xl">Sign in</CardTitle>
            <CardDescription>
              {brand.auth_mode === "oidc"
                ? "Use your organisation's SSO to continue."
                : "Enter your credentials to continue."}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {brand.auth_mode === "oidc" ? (
              <OIDCLoginButton />
            ) : (
              <LocalLoginForm redirectTo={from} />
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
