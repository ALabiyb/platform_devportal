// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query";
import { showToast } from "@/components/toast";

// Spread into a useMutation() whose callers all render the failure inline
// (try/catch → setError). Suppresses the global mutation-error toast so the
// user doesn't get the same message twice.
export const INLINE_ERRORS = { meta: { silentError: true } } as const;

function toastError(error: unknown, fallback: string) {
  showToast({
    tone: "bad",
    message: error instanceof ApiError ? error.message : fallback,
  });
}

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: (failureCount, error) => {
        if (error instanceof ApiError && error.status === 401) return false;
        return failureCount < 2;
      },
    },
  },
  // Global safety net: no page destructures `error` from its useQuery calls,
  // so without this a failed background fetch was completely invisible —
  // infinite spinner or a silently-empty list, no matter what actually broke.
  // Opt a query out with { meta: { silentError: true } } if it has its own
  // inline error handling already.
  queryCache: new QueryCache({
    onError: (error, query) => {
      if (query.meta?.silentError) return;
      // 401 triggers a silent redirect to sign-in (see apiFetch) — that's the
      // user-visible outcome, not a "something broke" toast on top of it.
      if (error instanceof ApiError && error.status === 401) return;
      toastError(error, "Failed to load data.");
    },
  }),
  // Same net for writes. Skipped when the hook opts out via INLINE_ERRORS or
  // supplies its own onError — either means a page is already showing feedback.
  mutationCache: new MutationCache({
    onError: (error, _vars, _ctx, mutation) => {
      if (mutation.meta?.silentError) return;
      if (mutation.options.onError) return;
      if (error instanceof ApiError && error.status === 401) return;
      toastError(error, "Request failed.");
    },
  }),
});

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string
  ) {
    super(message);
    this.name = "ApiError";
  }
}
