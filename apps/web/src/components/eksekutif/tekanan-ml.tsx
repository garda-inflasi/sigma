"use client";

import { useState } from "react";
import { PanelPas } from "@/components/garda/halaman-pas";
import { Istilah } from "@/components/garda/istilah";
import { Shimmer } from "@/components/garda/shimmer";
import { StatusBadge } from "@/components/garda/status-badge";
import { KurvaMiniMl } from "@/components/ml/forecast-ml";
import {
  CatatanBelumAndal,
  KosongMl,
  LabelTargetMl,
  PeringatanSiklusMl,
} from "@/components/ml/siklus-ml";
import type {
  BlokInflasiMl,
  BlokKesegaranMl,
  BlokMakroMl,
  BlokWilayahMl,
  InflasiWilayahMl,
  MakroMl,
} from "@/lib/api/ml";
import { useCan } from "@/lib/auth/session-context";
import { toneClass } from "@/lib/design/tokens";
import { formatAngka, formatPersen } from "@/lib/format";
import { titikHorizon } from "@/lib/ml/forecast";
import {
  alasanBelumAndal,
  dapatDiandalkan,
  keadaanBlok,
  LABEL_BELUM_ANDAL,
  LABEL_PERSISTENSI,
  LABEL_WILAYAH_TANPA_DATA,
  labelKesegaran,
  namaKomoditasMl,
  namaWilayahMl,
  sinyalTekanan,
  tanggalBisnis,
  type KeadaanBlok,
} from "@/lib/ml/label";
import { useLampiranMl } from "@/lib/query/ml-hooks";
import { cn } from "@/lib/utils";

const hurufKecilAwal = (t: string) => t.charAt(0).toLowerCase() + t.slice(1);
const pct = (v: number | null) =>
  v === null ? "–" : formatPersen(v, { signed: true, digits: 2 });

/**
 * Sinyal tekanan indeks harga pangan per wilayah dari siklus pilihan: status (atau "belum dapat diandalkan"),
 * perubahan indeks aktual, penyumbang, dan kurva ringkas proyeksi harga penyumbang utama. Indeks pangan tidak
 * pernah disebut inflasi; wilayah tanpa sinyal tidak dicantumkan.
 */
export function PanelTekananMl({
  inflasi,
  kesegaran,
  makro,
  wilayah,
  cycleId,
}: {
  inflasi: KeadaanBlok<BlokInflasiMl>;
  kesegaran: KeadaanBlok<BlokKesegaranMl>;
  makro?: KeadaanBlok<BlokMakroMl>;
  wilayah?: KeadaanBlok<BlokWilayahMl>;
  cycleId: string | undefined;
}) {
  const [pilih, setPilih] = useState<string>();
  const adaMakro = makro?.jenis === "tampil" && makro.blok.items.length > 0;
  const adaKlaster =
    wilayah?.jenis === "tampil" &&
    wilayah.blok.items.some((w) => w.klaster !== null);
  if (inflasi.jenis === "sembunyi" && !adaMakro && !adaKlaster) return null;
  if (inflasi.jenis === "memuat")
    return <Shimmer className="h-72 w-full shrink-0 rounded-lg" />;

  const blok = inflasi.jenis === "tampil" ? inflasi.blok : null;
  const daftarWilayah = blok?.wilayah ?? [];
  const aktif =
    daftarWilayah.find((w) => w.wilayahKode === pilih) ?? daftarWilayah[0];
  const andal = blok ? dapatDiandalkan(blok.kelayakan) : false;

  return (
    <PanelPas
      judul={
        blok || inflasi.jenis === "siklus"
          ? "Tekanan harga per wilayah"
          : "Indikator makro BPS & klaster wilayah"
      }
      deskripsi={
        blok ? (
          <LabelTargetMl target={blok.target} />
        ) : adaMakro ? (
          "Indikator BPS Aceh (70 tabel) & klaster K-Means hasil siklus ML"
        ) : (
          <LabelTargetMl target={null} />
        )
      }
      className="shrink-0 lg:max-h-none"
      isiClassName="lg:overflow-visible"
    >
      <div className="space-y-3">
        {inflasi.jenis === "siklus" && (
          <PeringatanSiklusMl pesan={inflasi.pesan} />
        )}
        {inflasi.jenis === "kosong" && <KosongMl pesan={inflasi.pesan} />}
        {blok && !andal && (
          <CatatanBelumAndal
            judul={LABEL_BELUM_ANDAL}
            alasan={`${alasanBelumAndal(blok.kelayakan, blok.alasanKelayakan)} Kelas, skor tekanan, dan siklus tidak ditampilkan sebagai prediksi.`}
          />
        )}
        {daftarWilayah.length > 0 && (
          <ul
            aria-label="Sinyal tekanan per wilayah"
            className="grid gap-2 sm:grid-cols-3"
          >
            {daftarWilayah.map((w) => (
              <li key={w.wilayahKode} className="min-w-0">
                <KartuWilayah
                  w={w}
                  aktif={w.wilayahKode === aktif?.wilayahKode}
                  onPilih={() => setPilih(w.wilayahKode)}
                />
              </li>
            ))}
          </ul>
        )}
        {aktif && <RincianWilayah w={aktif} cycleId={cycleId} />}
        {adaMakro && makro?.jenis === "tampil" && (
          <MakroRingkas blok={makro.blok} />
        )}
        {adaKlaster && wilayah?.jenis === "tampil" && (
          <KlasterRingkas blok={wilayah.blok} />
        )}
        {kesegaran.jenis === "tampil" && (
          <KesegaranRingkas blok={kesegaran.blok} />
        )}
      </div>
    </PanelPas>
  );
}

function KartuWilayah({
  w,
  aktif,
  onPilih,
}: {
  w: InflasiWilayahMl;
  aktif: boolean;
  onPilih: () => void;
}) {
  const s = sinyalTekanan(w);
  return (
    <button
      type="button"
      aria-pressed={aktif}
      onClick={onPilih}
      className={cn(
        "block h-full w-full rounded-lg border border-border px-3 py-2 text-left transition-colors hover:bg-secondary focus-visible:ring-[3px] focus-visible:ring-ring/30 focus-visible:outline-none",
        aktif && "border-navy-800 bg-secondary/60 dark:border-white",
      )}
    >
      <span className="flex items-center gap-1.5 text-[13px] font-medium">
        <span
          aria-hidden
          className={cn(
            "size-1.5 shrink-0 rounded-full",
            toneClass[s.tone].dot,
          )}
        />
        <span className="truncate">{namaWilayahMl(w.wilayahKode)}</span>
      </span>
      <span
        className="mt-0.5 block truncate text-xs text-muted-foreground"
        title={s.andal ? s.keterangan : undefined}
      >
        {s.andal ? s.judul : LABEL_BELUM_ANDAL}
      </span>
      <span className="num mt-1 block text-xs">
        <Istilah kunci="indeksPangan">Indeks aktual</Istilah> m-t-m{" "}
        {pct(w.aktual.mtm)} · y-o-y {pct(w.aktual.yoy)}
      </span>
    </button>
  );
}

function RincianWilayah({
  w,
  cycleId,
}: {
  w: InflasiWilayahMl;
  cycleId: string | undefined;
}) {
  const s = sinyalTekanan(w);
  const lihatPrediksi = useCan("prediksi.lihat");
  const utama = [...w.penyumbang]
    .filter((p) => p.andil !== null)
    .sort((a, b) => Math.abs(b.andil ?? 0) - Math.abs(a.andil ?? 0));
  const maks = Math.max(...utama.map((p) => Math.abs(p.andil ?? 0)), 1e-9);
  const lampiran = useLampiranMl(
    `/v1/prediksi?kabupaten=${encodeURIComponent(w.wilayahKode)}`,
    {
      cycleId,
      enabled: lihatPrediksi && w.tingkat === "KABUPATEN",
    },
  );
  const fr = keadaanBlok(
    lampiran.data,
    lampiran.data?.forecastRingkas,
    lampiran.isLoading,
  );
  const kurva =
    fr.jenis === "tampil"
      ? utama
          .slice(0, 3)
          .map((p) =>
            fr.blok.items.find((f) => f.komoditasKode === p.komoditasKode),
          )
          .filter((f) => !!f)
      : [];

  return (
    <div className="grid gap-3 rounded-lg border border-border p-3 md:grid-cols-2">
      <div className="min-w-0">
        <p className="text-[13px] font-medium">
          {namaWilayahMl(w.wilayahKode)}
        </p>
        <p className="text-xs text-muted-foreground">
          Indeks aktual per {tanggalBisnis(w.aktual.periode)}: m-t-m{" "}
          {pct(w.aktual.mtm)}, y-t-d {pct(w.aktual.ytd)}, y-o-y{" "}
          {pct(w.aktual.yoy)}
        </p>
        {s.andal ? (
          <p className="mt-1.5 flex flex-wrap items-center gap-1.5 text-xs">
            <StatusBadge tone={s.tone}>{s.judul}</StatusBadge>
            <span className="text-muted-foreground">{s.keterangan}</span>
          </p>
        ) : (
          <p className="mt-1.5 text-xs text-muted-foreground">
            <span className="font-medium text-foreground">
              {LABEL_BELUM_ANDAL}.
            </span>{" "}
            {w.probabilitas !== null && (
              <>
                Probabilitas mentah {formatAngka(w.probabilitas * 100, 0)}%
                dicatat untuk transparansi, bukan prediksi.
              </>
            )}
          </p>
        )}
        <p className="mt-3 text-xs font-medium">
          Penyumbang perubahan indeks (poin)
        </p>
        {utama.length === 0 ? (
          <p className="mt-1 text-xs text-muted-foreground">
            Tidak ada penyumbang tercatat.
          </p>
        ) : (
          <ul
            aria-label={`Penyumbang ${namaWilayahMl(w.wilayahKode)}`}
            className="mt-1.5 space-y-1.5"
          >
            {utama.slice(0, 5).map((p) => {
              const v = p.andil ?? 0;
              return (
                <li
                  key={p.komoditasKode}
                  className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-2 gap-y-1 text-xs"
                >
                  <span className="truncate" title={p.nama}>
                    {namaKomoditasMl(p.komoditasKode, p.nama)}
                  </span>
                  <span
                    className={cn(
                      "num font-semibold",
                      v >= 0
                        ? "text-defisit-ink dark:text-rose-300"
                        : "text-surplus-ink dark:text-blue-300",
                    )}
                  >
                    {v >= 0 ? "+" : "−"}
                    {formatAngka(Math.abs(v), 2)}
                  </span>
                  <span className="col-span-2 mb-0.5 h-1 rounded-full bg-mist-200 dark:bg-white/10">
                    <span
                      className={cn(
                        "block h-full rounded-full",
                        v >= 0 ? "bg-defisit" : "bg-surplus",
                      )}
                      style={{ width: `${(Math.abs(v) / maks) * 100}%` }}
                    />
                  </span>
                </li>
              );
            })}
          </ul>
        )}
      </div>
      <div className="min-w-0">
        <p className="text-xs font-medium">
          Kurva ringkas harga penyumbang utama (P50, pita P10–P90, 30 hari)
        </p>
        {!lihatPrediksi ? (
          <p className="mt-1 text-xs text-muted-foreground">
            Butuh izin melihat prediksi.
          </p>
        ) : fr.jenis === "memuat" ? (
          <Shimmer className="mt-2 h-28 w-full" />
        ) : kurva.length === 0 ? (
          <p className="mt-1 text-xs text-muted-foreground">
            {fr.jenis === "kosong" || fr.jenis === "sembunyi"
              ? LABEL_WILAYAH_TANPA_DATA
              : "Tidak ada forecast untuk penyumbang utama."}
          </p>
        ) : (
          <ul aria-label="Kurva ringkas" className="mt-1.5 space-y-2">
            {kurva.map((f) => {
              const t30 = titikHorizon(f.kurva, 30);
              return (
                <li
                  key={f.komoditasKode}
                  className="grid grid-cols-[minmax(0,7rem)_minmax(0,1fr)_auto] items-center gap-2 text-xs"
                >
                  <span className="truncate">
                    {namaKomoditasMl(f.komoditasKode)}
                  </span>
                  <KurvaMiniMl forecast={f} />
                  <span className="num text-muted-foreground">
                    {t30?.perubahanP50Pct === null ||
                    t30?.perubahanP50Pct === undefined
                      ? "–"
                      : formatPersen(t30.perubahanP50Pct, { signed: true })}
                    {f.model.adalahBaseline && (
                      <span title={LABEL_PERSISTENSI}>
                        {" "}
                        · persistensi
                      </span>
                    )}
                  </span>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </div>
  );
}

function formatNamaIndikatorMakro(indikator: string): string {
  return indikator
    .replace(/^indeks_harga_konsumen_bps_tbl_\d+_ihk_\d+_/i, "IHK ")
    .replace(/^inflasi_yoy_bps_tbl_\d+_inflasi_yoy_\d+_/i, "Inflasi YoY ")
    .replace(/^inflasi_ytd_bps_tbl_\d+_inflasi_ytd_\d+_/i, "Inflasi YtD ")
    .replace(/^bps_tbl_\d+_/i, "")
    .replaceAll("_", " ")
    .trim();
}

function nilaiMakroUtama(items: MakroMl[]): MakroMl[] {
  const urutan = ["INFLASI", "TENAGA_KERJA", "NERACA_EKONOMI", "KONSUMSI_PENDAPATAN"];
  const terurut = [...items].sort((a, b) => {
    const da = urutan.indexOf(a.domain);
    const db = urutan.indexOf(b.domain);
    if (da !== db) return (da === -1 ? 99 : da) - (db === -1 ? 99 : db);
    const ya = a.indikator.includes("inflasi_yoy") ? 0 : 1;
    const yb = b.indikator.includes("inflasi_yoy") ? 0 : 1;
    if (ya !== yb) return ya - yb;
    return (a.wilayahKode ?? "").localeCompare(b.wilayahKode ?? "");
  });
  const hasil: MakroMl[] = [];
  const terlihat = new Set<string>();
  for (const item of terurut) {
    if (item.nilai === null) continue;
    const kunci = `${item.domain}:${item.wilayahKode ?? "11"}`;
    if (terlihat.has(kunci) && item.domain !== "INFLASI") continue;
    terlihat.add(kunci);
    hasil.push(item);
    if (hasil.length >= 6) break;
  }
  return hasil;
}

function MakroRingkas({ blok }: { blok: BlokMakroMl }) {
  const sorotan = nilaiMakroUtama(blok.items);
  if (sorotan.length === 0) return null;
  const perDomain = new Map<string, number>();
  for (const item of blok.items) {
    perDomain.set(item.domain, (perDomain.get(item.domain) ?? 0) + 1);
  }
  return (
    <div className="border-t border-border pt-2.5">
      <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
        <p className="font-medium text-foreground">
          Indikator Makro BPS Aceh (70 tabel)
        </p>
        <p className="text-muted-foreground">
          {[...perDomain.entries()]
            .map(([d, n]) => `${d.replaceAll("_", " ").toLowerCase()}: ${n}`)
            .join(" · ")}
        </p>
      </div>
      <ul
        aria-label="Sorotan indikator makro BPS"
        className="mt-2 grid gap-2 sm:grid-cols-2 lg:grid-cols-3"
      >
        {sorotan.map((m, idx) => (
          <li
            key={`${m.domain}:${m.indikator}:${m.wilayahKode ?? "11"}:${idx}`}
            className="rounded-md border border-border bg-secondary/25 px-2.5 py-2 text-xs"
          >
            <p className="truncate font-medium text-foreground" title={m.indikator}>
              {formatNamaIndikatorMakro(m.indikator)}
            </p>
            <p className="truncate text-[11px] text-muted-foreground">
              {m.wilayahKode ? namaWilayahMl(m.wilayahKode) : "Aceh"}
              {m.periode ? ` · ${tanggalBisnis(m.periode)}` : ""}
            </p>
            <p className="num mt-1 text-sm font-semibold text-foreground">
              {m.nilai !== null ? formatAngka(m.nilai, 2) : "–"}{" "}
              <span className="text-[11px] font-normal text-muted-foreground">
                {m.satuan}
              </span>
            </p>
          </li>
        ))}
      </ul>
    </div>
  );
}

function KlasterRingkas({ blok }: { blok: BlokWilayahMl }) {
  const klasterItems = blok.items.filter((w) => w.klaster !== null);
  if (klasterItems.length === 0) return null;
  return (
    <div className="border-t border-border pt-2.5">
      <p className="text-xs font-medium text-foreground">
        Klaster Volatilitas Wilayah (K-Means)
      </p>
      <ul
        aria-label="Klaster volatilitas K-Means per wilayah"
        className="mt-2 flex flex-wrap gap-1.5"
      >
        {klasterItems.map((w) => {
          const k = w.klaster!;
          const tone = k.clusterLabel.toLowerCase().includes("tinggi") ||
            k.clusterLabel.toLowerCase().includes("gejolak")
            ? "defisit"
            : k.clusterLabel.toLowerCase().includes("sedang") ||
                k.clusterLabel.toLowerCase().includes("waspada")
              ? "waspada"
              : "normal";
          return (
            <li
              key={w.wilayahKode}
              className="inline-flex items-center gap-1.5 rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
            >
              <span className="font-medium">{namaWilayahMl(w.wilayahKode)}</span>
              <StatusBadge tone={tone}>{k.clusterLabel}</StatusBadge>
              <span className="num text-[11px] text-muted-foreground">
                σ {formatAngka(k.volatilityScore, 3)}
              </span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/** Kesegaran sumber data siklus; sumber basi, gagal, atau dilewati menyebut tanggal data asli terakhirnya. */
function KesegaranRingkas({ blok }: { blok: BlokKesegaranMl }) {
  const r = blok.ringkasan;
  const bermasalah = blok.sumber.filter(
    (s) => s.kategori === "BASI" || s.kategori === "GAGAL",
  );
  const dilewati = blok.sumber.filter((s) => s.kategori === "DILEWATI");
  return (
    <div className="border-t border-border pt-2.5 text-xs text-muted-foreground">
      <p>
        Kesegaran sumber: <span className="num text-foreground">{r.segar}</span>{" "}
        segar · <span className="num text-foreground">{r.basi}</span> basi ·{" "}
        <span className="num text-foreground">{r.gagal}</span> gagal ·{" "}
        <span className="num text-foreground">{r.dilewati}</span> dilewati
      </p>
      {(bermasalah.length > 0 || dilewati.length > 0) && (
        <ul className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5">
          {[...bermasalah, ...dilewati].slice(0, 6).map((s) => (
            <li key={`${s.sumber}:${s.dataset}`} title={s.pesan}>
              {s.sumber} {s.dataset.replaceAll("_", " ")}:{" "}
              {hurufKecilAwal(labelKesegaran(s).label)}
            </li>
          ))}
          {bermasalah.length + dilewati.length > 6 && (
            <li>+{bermasalah.length + dilewati.length - 6} lainnya</li>
          )}
        </ul>
      )}
    </div>
  );
}
