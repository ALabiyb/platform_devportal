// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
//
// RegisterPage is only reachable in local auth mode.
// It creates the first admin account via POST /auth/register.
// Subsequent user creation goes through the admin UI (Day 15).
import { useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import { UserPlus } from "lucide-react";
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
import { useRegister } from "@/lib/api";
import { ApiError } from "@/lib/queryClient";
import { useBrand } from "@/contexts/BrandContext";

interface FieldErrors {
  name?: string;
  email?: string;
  password?: string;
  confirm?: string;
}

const MIN_PASSWORD_LENGTH = 8;

function registerErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 409) return "An account with this email already exists.";
    if (error.status === 403) return "Registration is disabled — an admin account already exists.";
  }
  return "Something went wrong. Please try again.";
}

export function RegisterPage() {
  const navigate = useNavigate();
  const register = useRegister();
  const brand = useBrand();

  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});

  function clearError(field: keyof FieldErrors) {
    setFieldErrors((p) => ({ ...p, [field]: undefined }));
  }

  function validate(): FieldErrors {
    const errs: FieldErrors = {};
    if (!name.trim()) errs.name = "Full name is required.";
    if (!email.trim()) {
      errs.email = "Email is required.";
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      errs.email = "Enter a valid email address.";
    }
    if (!password) {
      errs.password = "Password is required.";
    } else if (password.length < MIN_PASSWORD_LENGTH) {
      errs.password = `Password must be at least ${MIN_PASSWORD_LENGTH} characters.`;
    }
    if (!confirm) {
      errs.confirm = "Please confirm your password.";
    } else if (confirm !== password) {
      errs.confirm = "Passwords do not match.";
    }
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
      await register.mutateAsync({ display_name: name.trim(), email, password });
      navigate("/", { replace: true });
    } catch {
      // error rendered below via register.error
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          {brand.logo_url ? (
            <img src={brand.logo_url} alt={brand.app_name} className="mx-auto mb-3 h-10 w-auto" />
          ) : (
            <h1 className="text-3xl font-bold tracking-tight text-primary">{brand.app_name}</h1>
          )}
          {brand.company && (
            <p className="mt-1 text-sm text-muted-foreground">{brand.company}</p>
          )}
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="text-xl">Create admin account</CardTitle>
            <CardDescription>
              Set up the first administrator account for this portal.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4" noValidate>
              <div className="space-y-1.5">
                <Label htmlFor="name">Full name</Label>
                <Input
                  id="name"
                  type="text"
                  autoComplete="name"
                  placeholder="Jane Smith"
                  value={name}
                  onChange={(e) => { setName(e.target.value); clearError("name"); }}
                  aria-invalid={!!fieldErrors.name}
                />
                {fieldErrors.name && (
                  <p className="text-xs text-destructive">{fieldErrors.name}</p>
                )}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="email">Email</Label>
                <Input
                  id="email"
                  type="email"
                  autoComplete="email"
                  placeholder="you@example.com"
                  value={email}
                  onChange={(e) => { setEmail(e.target.value); clearError("email"); }}
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
                  autoComplete="new-password"
                  value={password}
                  onChange={(e) => { setPassword(e.target.value); clearError("password"); }}
                  aria-invalid={!!fieldErrors.password}
                />
                {fieldErrors.password ? (
                  <p className="text-xs text-destructive">{fieldErrors.password}</p>
                ) : (
                  <p className="text-xs text-muted-foreground">
                    Minimum {MIN_PASSWORD_LENGTH} characters.
                  </p>
                )}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="confirm">Confirm password</Label>
                <Input
                  id="confirm"
                  type="password"
                  autoComplete="new-password"
                  value={confirm}
                  onChange={(e) => { setConfirm(e.target.value); clearError("confirm"); }}
                  aria-invalid={!!fieldErrors.confirm}
                />
                {fieldErrors.confirm && (
                  <p className="text-xs text-destructive">{fieldErrors.confirm}</p>
                )}
              </div>

              {register.isError && (
                <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {registerErrorMessage(register.error)}
                </p>
              )}

              <Button type="submit" className="w-full" disabled={register.isPending}>
                {register.isPending ? (
                  <span className="flex items-center gap-2">
                    <span className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
                    Creating account…
                  </span>
                ) : (
                  <span className="flex items-center gap-2">
                    <UserPlus className="h-4 w-4" />
                    Create admin account
                  </span>
                )}
              </Button>

              <p className="text-center text-xs text-muted-foreground">
                Already have an account?{" "}
                <Link to="/login" className="text-primary underline-offset-4 hover:underline">
                  Sign in
                </Link>
              </p>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
