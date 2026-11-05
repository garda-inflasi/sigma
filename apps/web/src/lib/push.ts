/**
 * Web Push (PWA) di sisi browser: meminta izin, berlangganan lewat service worker, dan
 * mendaftarkan langganan ke API. Pengiriman nyata hanya terjadi bila server memiliki kunci VAPID.
 */

import { api } from "@/lib/api/endpoints";
import { kunciVapidKeBytes, toLanggananPushBody } from "@/lib/api/ews";

export type HasilPush = "aktif" | "ditolak" | "tidak-didukung";

export function pushDidukung(): boolean {
  return (
    typeof window !== "undefined" &&
    "serviceWorker" in navigator &&
    "PushManager" in window &&
    "Notification" in window
  );
}

/** Berlangganan Web Push di perangkat ini lalu mendaftarkannya ke server. Galat jaringan dilempar. */
export async function aktifkanPush(vapidPublicKey: string): Promise<HasilPush> {
  if (!pushDidukung()) return "tidak-didukung";
  if ((await Notification.requestPermission()) !== "granted") return "ditolak";

  await navigator.serviceWorker.register("/sw.js", { scope: "/" });
  const registrasi = await navigator.serviceWorker.ready;
  const langganan =
    (await registrasi.pushManager.getSubscription()) ??
    (await registrasi.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: kunciVapidKeBytes(vapidPublicKey) as BufferSource,
    }));

  const body = toLanggananPushBody(
    langganan.toJSON() as { endpoint?: string; keys?: Record<string, string> },
  );
  if (!body) throw new Error("Langganan push dari browser tidak lengkap.");
  await api.ews.daftarPush(body);
  return "aktif";
}

export type HasilLepasPush = "dilepas" | "tidak-ada" | "tidak-didukung";

/**
 * Melepas langganan Web Push perangkat ini: server dibatalkan lebih dulu, baru langganan browser
 * dihapus, sehingga bila server gagal pengguna masih bisa mengulang. Tanpa langganan di
 * perangkat ini tidak ada yang dibatalkan ("tidak-ada").
 */
export async function nonaktifkanPush(): Promise<HasilLepasPush> {
  if (!pushDidukung()) return "tidak-didukung";
  const registrasi = await navigator.serviceWorker.getRegistration("/");
  const langganan = await registrasi?.pushManager.getSubscription();
  if (!langganan) return "tidak-ada";

  await api.ews.batalPush(langganan.endpoint);
  await langganan.unsubscribe();
  return "dilepas";
}
