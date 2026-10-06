import type { Metadata, Viewport } from "next";
import { Inter, JetBrains_Mono, Plus_Jakarta_Sans } from "next/font/google";
import { KonturBackdrop } from "@/components/garda/kontur-backdrop";
import { getSession } from "@/lib/auth/session";
import { Providers } from "./providers";
import "./globals.css";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  display: "swap",
});
const jakarta = Plus_Jakarta_Sans({
  subsets: ["latin"],
  variable: "--font-jakarta",
  weight: ["500", "600", "700", "800"],
  display: "swap",
});
const mono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-jetbrains",
  weight: ["400", "500", "600"],
  display: "swap",
});

export const metadata: Metadata = {
  title: { default: "AI Garda Inflasi", template: "%s — Garda Inflasi" },
  description:
    "Platform terintegrasi pengendalian inflasi daerah berbasis data dan rantai pasok — FMIPA USK × Kanwil DJPb.",
  applicationName: "Garda Inflasi",
  appleWebApp: {
    capable: true,
    title: "Garda Lapangan",
    statusBarStyle: "default",
  },
  // Tanpa ini browser jatuh ke /favicon.ico yang tidak ada → 404 di konsol.
  icons: {
    icon: [
      { url: "/icons/icon-192.png", sizes: "192x192", type: "image/png" },
      { url: "/icons/icon-512.png", sizes: "512x512", type: "image/png" },
    ],
    apple: [
      { url: "/icons/icon-192.png", sizes: "192x192", type: "image/png" },
    ],
  },
};

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#f1f5f9" },
    { media: "(prefers-color-scheme: dark)", color: "#08112a" },
  ],
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
};

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const user = await getSession();
  return (
    <html
      lang="id"
      data-siklus="normal"
      suppressHydrationWarning
      className={`${inter.variable} ${jakarta.variable} ${mono.variable}`}
    >
      <body>
        <a
          href="#konten"
          className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:rounded-xl focus:bg-white focus:px-4 focus:py-2"
        >
          Lewati ke konten
        </a>
        <KonturBackdrop />
        <Providers user={user}>{children}</Providers>
      </body>
    </html>
  );
}
