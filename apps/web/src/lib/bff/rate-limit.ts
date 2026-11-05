/**
 * Rate limiting utilities for the BFF layer.
 * Uses an in-memory sliding window per IP.
 * Note: For production, use Redis or a distributed store.
 */

interface RateLimitEntry {
  timestamps: number[];
}

const store = new Map<string, RateLimitEntry>();

// Cleanup old entries every 5 minutes. unref(): pembersih tidak boleh menahan proses (mis. proses tes) tetap hidup.
const pembersih = setInterval(() => {
  const now = Date.now();
  for (const [key, entry] of store.entries()) {
    entry.timestamps = entry.timestamps.filter((t) => now - t < 60_000);
    if (entry.timestamps.length === 0) {
      store.delete(key);
    }
  }
}, 5 * 60 * 1000);
(pembersih as { unref?: () => void }).unref?.();

/**
 * Checks if a request should be rate limited.
 * Returns true if the request is allowed, false if rate limited.
 */
export function checkRateLimit(
  key: string,
  limit: number = 30,
  windowMs: number = 60_000
): boolean {
  return ukurBatasLaju(key, limit, windowMs).diizinkan;
}

/** Hasil pemeriksaan batas laju: bila ditolak, `tungguDetik` adalah lama sampai percobaan berikutnya boleh. */
export interface HasilBatasLaju {
  diizinkan: boolean;
  tungguDetik: number;
}

/**
 * Seperti checkRateLimit tetapi juga melaporkan berapa detik lagi sampai permintaan boleh lewat (untuk
 * header Retry-After dan hitung mundur di form). Permintaan yang diizinkan dicatat; yang ditolak tidak.
 */
export function ukurBatasLaju(
  key: string,
  limit: number = 30,
  windowMs: number = 60_000,
  now: number = Date.now()
): HasilBatasLaju {
  const entry = store.get(key) || { timestamps: [] };

  // Remove timestamps outside the window
  entry.timestamps = entry.timestamps.filter(
    (t) => now - t < windowMs
  );

  if (entry.timestamps.length >= limit) {
    // Permintaan baru lolos begitu isi jendela turun di bawah limit: entri ke-(n - limit) dari yang tertua harus habis.
    const penahan = entry.timestamps[entry.timestamps.length - limit];
    store.set(key, entry);
    return {
      diizinkan: false,
      tungguDetik: Math.max(1, Math.ceil((penahan + windowMs - now) / 1000)),
    };
  }

  entry.timestamps.push(now);
  store.set(key, entry);
  return { diizinkan: true, tungguDetik: 0 };
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
  return ukurBatasLogin(request).diizinkan;
}

/** Batas login (5/menit/IP) beserta lama tunggu bila ditolak. Dipakai bersama oleh login dan validasi OTP. */
export function ukurBatasLogin(request: Request): HasilBatasLaju {
  return ukurBatasLaju(getRateLimitKey(request), 5, 60_000);
}
