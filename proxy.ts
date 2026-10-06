/**
 * Next.js 16 proxy (pengganti middleware): gerbang RBAC tingkat rute.
 * Penegakan sesungguhnya tetap di API Go (defense in depth) — proxy hanya
 * mencegah navigasi ke halaman yang tidak diizinkan.
 */
import { NextResponse, type NextRequest } from "next/server";
import { resolveSession } from "@/lib/auth/resolve";
import { can } from "@/lib/rbac/permissions";
import { requiredPermission } from "@/lib/rbac/routes";
import { ROLES } from "@/lib/rbac/roles";

const PUBLIC = ["/masuk", "/lupa-password", "/akses-ditolak", "/offline"];

export async function proxy(req: NextRequest) {
  const { pathname } = req.nextUrl;
  if (PUBLIC.some((p) => pathname.startsWith(p))) return NextResponse.next();

  const user = await resolveSession((n) => req.cookies.get(n)?.value);
  if (!user) {
    const url = new URL("/masuk", req.url);
    url.searchParams.set("lanjut", pathname);
    return NextResponse.redirect(url);
  }

  if (pathname === "/")
    return NextResponse.redirect(new URL(ROLES[user.role].beranda, req.url));

  const perm = requiredPermission(pathname);
  if (perm && !can(user.role, perm)) {
    const url = new URL("/akses-ditolak", req.url);
    url.searchParams.set("dari", pathname);
    return NextResponse.redirect(url);
  }
  return NextResponse.next();
}

export const config = {
  matcher: [
    "/((?!_next/static|_next/image|favicon.ico|sw.js|manifest.webmanifest|icons/|api/).*)",
  ],
};
