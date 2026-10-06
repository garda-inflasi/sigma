/* Service worker Garda Lapangan — app shell offline-first.
   Navigasi: network-first → cache → /offline. Aset statis: cache-first. API: tidak di-cache
   (antrean data ditangani IndexedDB + sync engine di klien). */
const VERSI = "garda-v1";
const SHELL = ["/lapangan/input", "/lapangan/riwayat", "/lapangan/tugas", "/offline", "/manifest.webmanifest"];

self.addEventListener("install", (e) => {
  e.waitUntil(caches.open(VERSI).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()));
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
          const copy = res.clone();
          caches.open(VERSI).then((c) => c.put(request, copy));
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
