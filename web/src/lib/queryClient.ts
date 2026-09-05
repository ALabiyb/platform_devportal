// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { QueryCache, QueryClient } from "@tanstack/react-query";
import { showToast } from "@/components/toast";

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
      showToast({
        tone: "bad",
        message: error instanceof ApiError ? error.message : "Failed to load data.",
      });
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
