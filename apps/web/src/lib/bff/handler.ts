/**
 * Shared helpers for BFF Route Handlers.
 * Centralises session lookup, CSRF/origin validation, and error mapping so
 * every route handler behaves consistently.
 */

import type { NextRequest } from "next/server";
import type { NextResponse } from "next/server";
import { getAccessToken } from "./cookies";
import { validateCSRF, validateOrigin } from "./csrf";
import { errorResponse } from "./security-headers";

type SessionResult =
  | { accessToken: string; error?: undefined }
  | { accessToken?: undefined; error: NextResponse };

/**
 * Resolves the caller's access token, optionally enforcing CSRF + origin
 * checks for state-changing requests.
 */
export async function requireSession(
  request: NextRequest,
  opts: { csrf?: boolean } = {}
): Promise<SessionResult> {
  if (opts.csrf) {
    if (!(await validateCSRF(request)) || !validateOrigin(request)) {
      return { error: errorResponse("CSRF token tidak valid.", 403) };
    }
  }

  const accessToken = await getAccessToken();
  if (!accessToken) {
    return { error: errorResponse("Tidak terautentikasi.", 401) };
  }

  return { accessToken };
}

/** Maps a thrown backend error onto a BFF error response. */
export function handleRouteError(error: unknown): NextResponse {
  if (error instanceof Error && "status" in error) {
    const apiError = error as { status: number; message: string };
    return errorResponse(apiError.message, apiError.status);
  }
  return errorResponse("Kesalahan server internal.", 500);
}