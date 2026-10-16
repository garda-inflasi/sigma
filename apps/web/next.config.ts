import type { NextConfig } from "next";

/**
 * Next.js 16 — Turbopack default, `proxy.ts` menggantikan middleware.
 * Header keamanan dipasang di sini (security by design, selaras SPBE).
 */
const securityHeaders = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "X-Frame-Options", value: "DENY" },
  // Kamera & GPS hanya untuk origin sendiri (mobile app tim lapangan)
  { key: "Permissions-Policy", value: "camera=(self), geolocation=(self), microphone=()" },
  // NOTE: Strict-Transport-Security is intentionally NOT set here.
  // Next.js cannot tell whether the request arrived over TLS (Caddy
  // terminates it), so emitting HSTS here would also send it over plain
  // HTTP — which makes the browser rewrite http://localhost to
  // https://localhost and breaks the app. Caddy owns this header and
  // emits it only for HTTPS requests.
];

const nextConfig: NextConfig = {
  output: "standalone",
  poweredByHeader: false,
  experimental: {
    optimizePackageImports: ["lucide-react", "recharts", "date-fns"],
  },
  async headers() {
    return [
      { source: "/:path*", headers: securityHeaders },
      {
        source: "/sw.js",
        headers: [
          { key: "Cache-Control", value: "no-cache, no-store, must-revalidate" },
          { key: "Service-Worker-Allowed", value: "/" },
        ],
      },
    ];
  },
};

export default nextConfig;
