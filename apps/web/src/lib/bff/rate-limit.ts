/**
 * Rate limiting utilities for the BFF layer.
 * Uses an in-memory sliding window per IP.
 * Note: For production, use Redis or a distributed store.
 */

interface RateLimitEntry {
  timestamps: number[];
}

const store = new Map<string, RateLimitEntry>();

// Cleanup old entries every 5 minutes
setInterval(() => {
  const now = Date.now();
  for (const [key, entry] of store.entries()) {
    entry.timestamps = entry.timestamps.filter((t) => now - t < 60_000);
    if (entry.timestamps.length === 0) {
      store.delete(key);
    }
  }
}, 5 * 60 * 1000);

/**
 * Checks if a request should be rate limited.
 * Returns true if the request is allowed, false if rate limited.
 */
export function checkRateLimit(
  key: string,
  limit: number = 30,
  windowMs: number = 60_000
): boolean {
  const now = Date.now();
  const entry = store.get(key) || { timestamps: [] };

  // Remove timestamps outside the window
  entry.timestamps = entry.timestamps.filter(
    (t) => now - t < windowMs
  );

  if (entry.timestamps.length >= limit) {
    return false;
  }

  entry.timestamps.push(now);
  store.set(key, entry);
  return true;
}

/**
 * Rate limit key from request (IP-based).
 */
export function getRateLimitKey(request: Request): string {
  const forwarded = request.headers.get("x-forwarded-for");
  const ip = forwarded?.split(",")[0]?.trim() || "127.0.0.1";
  return `bff:${ip}`;
}

/**
 * Strict rate limit for login endpoint.
 */
export function checkLoginRateLimit(request: Request): boolean {
  return checkRateLimit(getRateLimitKey(request), 5, 60_000);
}
