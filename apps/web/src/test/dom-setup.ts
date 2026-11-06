/**
 * Penyiapan lingkungan tes komponen (`pnpm test:komponen`): DOM happy-dom sebagai global Node, dimuat lewat
 * `node --import` SEBELUM berkas tes mana pun, jadi modul aplikasi yang membaca `window`/`document`/`process.env`
 * saat dimuat melihat lingkungan yang sama dengan di browser.
 *
 * Tes komponen berjalan TANPA `--conditions=react-server` (React harus resolve ke build klien), terpisah dari
 * tes logika `src/**\/*.test.ts` yang memang butuh kondisi itu untuk `server-only`.
 */
import { GlobalRegistrator } from "@happy-dom/global-registrator";

// Mode API nyata: tanpa ini lib/api/client.ts memilih data mock berpenundaan (MODE_MOCK). Tes menyadap fetch,
// jadi alamat ini tidak pernah dihubungi.
process.env.NEXT_PUBLIC_API_BASE_URL ??= "http://api.test";

GlobalRegistrator.register({
  url: "http://localhost:3000/pengguna",
  width: 1280,
  height: 800,
});

// React: tanpa ini `act()` memperingatkan pembaruan di luar act pada setiap render tes.
(
  globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }
).IS_REACT_ACT_ENVIRONMENT = true;

// Radix (Select, DropdownMenu) memanggil API pointer/scroll yang belum dimiliki happy-dom.
const proto = window.HTMLElement.prototype as unknown as Record<
  string,
  unknown
>;
proto.hasPointerCapture ??= () => false;
proto.setPointerCapture ??= () => {};
proto.releasePointerCapture ??= () => {};
proto.scrollIntoView ??= () => {};

// motion memakai Web Animations API bila tersedia. Animasi native happy-dom ditolak dengan AbortError saat
// dibatalkan (unmount/keluar) dan bocor sebagai unhandledRejection ke tes lain; tanpa `animate`, motion kembali
// ke animasi JS yang tidak memerlukan DOM sungguhan.
delete (window.Element.prototype as unknown as Record<string, unknown>).animate;

// Jangan biarkan locale bawaan mesin memengaruhi `toLocaleString("id-ID")` di tes: Node memakai ICU penuh.
process.env.TZ ??= "UTC";
