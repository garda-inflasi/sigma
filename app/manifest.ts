import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "Garda Inflasi — Lapangan",
    short_name: "Garda Lapangan",
    description: "Pencatatan harga komoditas offline-first untuk agen lapangan Garda Inflasi.",
    start_url: "/lapangan/input",
    scope: "/",
    display: "standalone",
    orientation: "portrait",
    background_color: "#f1f5f9",
    theme_color: "#1a2f5e",
    lang: "id",
    icons: [
      { src: "/icons/icon-192.png", sizes: "192x192", type: "image/png" },
      { src: "/icons/icon-512.png", sizes: "512x512", type: "image/png" },
      { src: "/icons/icon-maskable-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
    ],
    shortcuts: [
      { name: "Input harga", url: "/lapangan/input" },
      { name: "Tugas survei", url: "/lapangan/tugas" },
    ],
  };
}
