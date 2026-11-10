/**
 * Pelacak kunci untuk tes kontrak fixture. Data JSON dibungkus Proxy yang mencatat setiap properti yang dibaca pemeta;
 * hasilnya dibandingkan dengan semua kunci yang ada di data.
 *
 * - Kunci di data yang tidak pernah dibaca = field yang belum dikenali pemeta (field baru backend, atau salah ketik).
 *   Sengaja tidak dipakai UI? Daftarkan di `abaikan` beserta alasannya, jadi keputusannya tertulis.
 * - Kunci yang dibaca pemeta padahal tidak ada di data = pemeta menunggu field yang tidak dikirim backend.
 *
 * Jalur kunci memakai titik untuk objek dan `[]` untuk item larik: `rute.items[].asal_kode`.
 */

/** Semua jalur kunci objek di dalam data. */
export function jalurKunci(data: unknown): Set<string> {
  const hasil = new Set<string>();
  const telusuri = (nilai: unknown, jalur: string) => {
    if (Array.isArray(nilai)) {
      for (const item of nilai) telusuri(item, `${jalur}[]`);
    } else if (nilai && typeof nilai === "object") {
      for (const [kunci, isi] of Object.entries(nilai)) {
        const sub = jalur ? `${jalur}.${kunci}` : kunci;
        hasil.add(sub);
        telusuri(isi, sub);
      }
    }
  };
  telusuri(data, "");
  return hasil;
}

/** Properti yang dibaca mesin JS sendiri (bukan pemeta) saat objek diserialisasi atau dibandingkan. */
const BACAAN_MESIN = new Set([
  "toJSON",
  "then",
  "constructor",
  "toString",
  "valueOf",
  "asymmetricMatch",
  "$$typeof",
  "nodeType",
  "tagName",
]);

export interface JejakBaca<T> {
  /** Salinan data yang dilacak: berikan ke pemeta. */
  data: T;
  /** Jalur kunci yang dibaca dan memang ada di data. */
  terbaca: Set<string>;
  /** Jalur kunci yang dibaca padahal tidak ada di data. */
  tidakAda: Set<string>;
}

/** Membungkus data dengan pencatat bacaan. Hasil pemeta yang meneruskan objek apa adanya dibaca lewat JSON.stringify. */
export function lacakBaca<T>(data: T): JejakBaca<T> {
  const terbaca = new Set<string>();
  const tidakAda = new Set<string>();

  const bungkus = (nilai: unknown, jalur: string): unknown => {
    if (!nilai || typeof nilai !== "object") return nilai;
    const larik = Array.isArray(nilai);
    return new Proxy(nilai, {
      get(target, prop, receiver) {
        const isi = Reflect.get(target, prop, receiver);
        if (typeof prop === "symbol") return isi;
        if (larik) {
          // Indeks larik: item sejenis berbagi jalur `[]`; metode dan `length` bukan kunci data.
          return /^\d+$/.test(prop) ? bungkus(isi, `${jalur}[]`) : isi;
        }
        if (BACAAN_MESIN.has(prop)) return isi;
        const sub = jalur ? `${jalur}.${prop}` : prop;
        if (Object.hasOwn(target, prop)) terbaca.add(sub);
        else tidakAda.add(sub);
        return bungkus(isi, sub);
      },
    });
  };

  return { data: bungkus(data, "") as T, terbaca, tidakAda };
}

export interface HasilKontrak<U> {
  hasil: U;
  /** Kunci fixture yang tidak dibaca pemeta dan tidak diabaikan. */
  takTerpetakan: string[];
  /** Kunci yang dibaca pemeta tetapi tidak ada di fixture. */
  takAda: string[];
  /** Entri `abaikan` yang ternyata dibaca pemeta atau tidak ada lagi di fixture (daftar basi). */
  abaikanBasi: string[];
}

/**
 * Menjalankan pemeta atas fixture dan membandingkan kunci yang dibacanya dengan kunci fixture.
 * `abaikan`: jalur kunci yang sengaja tidak dipakai UI.
 */
export function periksaKontrak<T, U>(
  fixture: T,
  petakan: (data: T) => U,
  abaikan: readonly string[] = [],
): HasilKontrak<U> {
  const jejak = lacakBaca(fixture);
  const hasil = petakan(jejak.data);
  // Objek yang diteruskan apa adanya ke hasil (tanpa disalin pemeta) ikut dibaca di sini.
  JSON.stringify(hasil);
  const semua = jalurKunci(fixture);
  const diabaikan = new Set(abaikan);
  return {
    hasil,
    takTerpetakan: [...semua]
      .filter((k) => !jejak.terbaca.has(k) && !diabaikan.has(k))
      .sort(),
    takAda: [...jejak.tidakAda].sort(),
    abaikanBasi: abaikan
      .filter((k) => jejak.terbaca.has(k) || !semua.has(k))
      .sort(),
  };
}

/** Jalur nilai `undefined`, `NaN`, atau tak hingga di dalam hasil pemeta (tidak boleh ada di model UI). */
export function nilaiTakWajar(data: unknown, jalur = "$"): string[] {
  if (data === undefined) return [jalur];
  if (typeof data === "number")
    return Number.isFinite(data) ? [] : [`${jalur}=${data}`];
  if (Array.isArray(data))
    return data.flatMap((x, i) => nilaiTakWajar(x, `${jalur}[${i}]`));
  if (data && typeof data === "object")
    return Object.entries(data).flatMap(([k, v]) =>
      nilaiTakWajar(v, `${jalur}.${k}`),
    );
  return [];
}
