/**
 * Safe redirect utility to prevent open redirect vulnerabilities.
 *
 * Only allows redirects to:
 * 1. Relative URLs (starts with /)
 * 2. Same-origin absolute URLs
 * 3. Explicitly allowed external URLs
 */

import "server-only";

// eslint-disable-next-line no-control-regex
const KARAKTER_KONTROL = /[\u0000-\u001f\u007f]/;

const ALLOWED_EXTERNAL_HOSTS = new Set<string>([
  // Add any allowed external redirect targets here
]);

/**
 * Validates that a redirect URL is safe.
 * Returns the safe URL string or null if the URL is unsafe.
 */
export function validateRedirectUrl(url: string | null): string | null {
  if (!url) return null;

  // Allow relative URLs
  if (url.startsWith("/")) {
    // Satu garis miring di depan saja yang berarti "path di origin ini". `//host` (protocol-relative),
    // `/\host` (peramban menganggap "\" sama dengan "/"), dan karakter kontrol (peramban membuang tab/
    // newline sehingga `/<TAB>/host` menjadi `//host`) semuanya mengalihkan ke origin lain. Garis miring
    // ganda di tengah path juga ditolak (tidak pernah dibutuhkan tujuan internal).
    if (
      url.includes("//") ||
      url.includes("\\") ||
      KARAKTER_KONTROL.test(url)
    ) {
      return null;
    }
    return url;
  }

  try {
    const parsed = new URL(url);

    // Block javascript: and data: URLs
    if (parsed.protocol === "javascript:" || parsed.protocol === "data:") {
      return null;
    }

    // Allow same-origin URLs
    const appUrl = process.env.NEXT_PUBLIC_APP_URL || "http://localhost:3000";
    if (parsed.origin === new URL(appUrl).origin) {
      return url;
    }

    // Allow explicitly whitelisted external hosts
    if (ALLOWED_EXTERNAL_HOSTS.has(parsed.host)) {
      return url;
    }

    return null;
  } catch {
    // Invalid URL — not safe to redirect
    return null;
  }
}

/**
 * Creates a safe redirect URL from a search parameter.
 * Falls back to the default path if the redirect URL is unsafe.
 */
export function getSafeRedirectUrl(
  searchParams: URLSearchParams,
  paramName: string = "redirect",
  fallback: string = "/"
): string {
  const redirectParam = searchParams.get(paramName);
  const safeUrl = validateRedirectUrl(redirectParam);
  return safeUrl ?? fallback;
}
