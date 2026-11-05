"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { authApi } from "@/lib/bff/api-client";
import {
  ACCESS_COOKIE,
  CSRF_COOKIE,
  REFRESH_COOKIE,
  cookieSecure,
} from "@/lib/bff/cookie-policy";
import { generateCSRFToken } from "@/lib/bff/csrf";
import { postLoginDestination } from "@/lib/auth/password-change";
import { safeInternalPath } from "@/lib/auth/safe-path";

export async function loginAction(formData: FormData) {
  const email = formData.get("email") as string;
  const password = formData.get("password") as string;
  const redirectTo = safeInternalPath(
    formData.get("redirect") as string | null,
  );

  if (!email || !password) {
    return { error: "Email dan password wajib diisi." };
  }

  try {
    const result = await authApi.login({ email, password });

    // Set cookies on the response via the cookie store
    const cookieStore = await cookies();

    // Access token: 15 menit
    cookieStore.set(ACCESS_COOKIE, result.access_token, {
      httpOnly: true,
      secure: cookieSecure(),
      sameSite: "lax",
      path: "/",
      maxAge: 15 * 60,
    });

    // Refresh token: 7 hari
    cookieStore.set(REFRESH_COOKIE, result.refresh_token, {
      httpOnly: true,
      secure: cookieSecure(),
      sameSite: "strict",
      path: "/api/auth",
      maxAge: 7 * 24 * 60 * 60,
    });

    // CSRF token: 24 jam
    const csrfToken = await generateCSRFToken();
    cookieStore.set(CSRF_COOKIE, csrfToken, {
      httpOnly: false,
      secure: cookieSecure(),
      sameSite: "lax",
      path: "/",
      maxAge: 24 * 60 * 60,
    });

    // Akun yang wajib mengganti kata sandi dialihkan dulu ke halaman ganti kata sandi
    redirect(
      postLoginDestination(result.user.wajib_ganti_password, redirectTo),
    );
  } catch (error) {
    const message =
      error instanceof Error && "status" in error
        ? (error as { status: number; message: string }).message
        : "Kesalahan server internal.";
    return { error: message };
  }
}

// Wrapper for form action (returns void/Promise<void>)
export async function loginFormAction(formData: FormData): Promise<void> {
  const result = await loginAction(formData);
  if (result && "error" in result) {
    throw new Error(result.error);
  }
}
