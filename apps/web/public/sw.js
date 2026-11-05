/* Service worker Garda Lapangan — app shell offline-first.
   Navigasi: network-first → cache → /offline. Aset statis: cache-first. API: tidak di-cache
   (antrean data ditangani IndexedDB + sync engine di klien). */
const VERSI = "garda-v1";
// Halaman berpengguna: HTML dirender server bersama sesi (nama, peran, wilayah). Dihapus dari cache saat pengguna
// keluar atau berganti (src/lib/offline/bersihkan.ts); harus tetap selaras dengan aturan di sana.
const SHELL = ["/lapangan/input", "/lapangan/riwayat", "/lapangan/tugas"];
// Halaman publik: diambil TANPA cookie karena layout akar menyisipkan sesi ke HTML, sehingga salinan yang diambil
// saat masuk akan memuat data pengguna. Tanpa cookie salinannya anonim dan aman dipertahankan saat keluar.
const SHELL_PUBLIK = ["/offline", "/manifest.webmanifest"];

self.addEventListener("install", (e) => {
  e.waitUntil(
    caches
      .open(VERSI)
      .then(async (c) => {
        await c.addAll(SHELL);
        for (const u of SHELL_PUBLIK) {
          try {
            const res = await fetch(u, { credentials: "omit" });
            if (res.ok && !res.redirected) await c.put(u, res);
          } catch {
            // Halaman publik gagal diambil tidak boleh menggagalkan pemasangan; dicoba lagi pada pembaruan berikutnya.
          }
        }
      })
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (e) => {
  e.waitUntil(
    caches.keys().then((keys) => Promise.all(keys.filter((k) => k !== VERSI).map((k) => caches.delete(k)))).then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (e) => {
  const { request } = e;
  if (request.method !== "GET") return;
  const url = new URL(request.url);
  if (url.origin !== self.location.origin || url.pathname.startsWith("/api/")) return;

  if (request.mode === "navigate") {
    e.respondWith(
      fetch(request)
        .then((res) => {
          // /offline hanya boleh berisi salinan anonim dari pemasangan; navigasi langsung membawa cookie sesi.
          if (url.pathname !== "/offline") {
            const copy = res.clone();
            caches.open(VERSI).then((c) => c.put(request, copy));
          }
          return res;
        })
        .catch(async () => (await caches.match(request)) || (await caches.match("/offline"))),
    );
    return;
  }

  if (url.pathname.startsWith("/_next/static/") || url.pathname.startsWith("/icons/") || /\.(woff2|png|svg|css|js)$/.test(url.pathname)) {
    e.respondWith(
      caches.match(request).then(
        (hit) =>
          hit ||
          fetch(request).then((res) => {
            const copy = res.clone();
            caches.open(VERSI).then((c) => c.put(request, copy));
            return res;
          }),
      ),
    );
  }
});

/* Background Sync: minta klien menjalankan sync engine saat koneksi pulih. */
self.addEventListener("sync", (e) => {
  if (e.tag === "garda-sync") {
    e.waitUntil(self.clients.matchAll().then((cs) => cs.forEach((c) => c.postMessage("garda:sinkron"))));
  }
});

/* Web Push: notifikasi alert EWS (payload JSON {title, body, url, tag} dari server). */
self.addEventListener("push", (e) => {
  let pesan = {};
  try {
    pesan = e.data ? e.data.json() : {};
  } catch {
    pesan = { body: e.data ? e.data.text() : "" };
  }
  e.waitUntil(
    self.registration.showNotification(pesan.title || "Garda Inflasi", {
      body: pesan.body || "",
      tag: pesan.tag,
      icon: "/icons/icon-192.png",
      data: { url: pesan.url || "/ews" },
    }),
  );
});

self.addEventListener("notificationclick", (e) => {
  e.notification.close();
  const url = (e.notification.data && e.notification.data.url) || "/ews";
  e.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((cs) => {
      const buka = cs.find((c) => "focus" in c);
      return buka ? buka.navigate(url).then((c) => c && c.focus()) : self.clients.openWindow(url);
    }),
  );
});
