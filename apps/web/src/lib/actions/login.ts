"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { authApi } from "@/lib/bff/api-client";
import { generateCSRFToken } from "@/lib/bff/csrf";

export async function loginAction(formData: FormData) {
  const email = formData.get("email") as string;
  const password = formData.get("password") as string;
  const redirectTo = (formData.get("redirect") as string) || "/";

  if (!email || !password) {
    return { error: "Email dan password wajib diisi." };
  }

  try {
    const result = await authApi.login({ email, password });

    // Set cookies on the response via the cookie store
    const cookieStore = await cookies();

    // Access token: 15 menit
    cookieStore.set("garda_access", result.access_token, {
      httpOnly: true,
      secure: false,
      sameSite: "lax",
      path: "/",
      maxAge: 15 * 60,
    });

    // Refresh token: 7 hari
    cookieStore.set("garda_refresh", result.refresh_token, {
      httpOnly: true,
      secure: false,
      sameSite: "strict",
      path: "/api/auth",
      maxAge: 7 * 24 * 60 * 60,
    });

    // CSRF token: 24 jam
    const csrfToken = await generateCSRFToken();
    cookieStore.set("garda_csrf", csrfToken, {
      httpOnly: false,
      secure: false,
      sameSite: "lax",
      path: "/",
      maxAge: 24 * 60 * 60,
    });

    // Redirect to the intended page
    redirect(redirectTo);
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
