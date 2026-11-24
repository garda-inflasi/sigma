"use client";

import type { ColumnDef } from "@tanstack/react-table";
import {
  Archive,
  ArrowRightLeft,
  ClipboardCheck,
  Cpu,
  Download,
  FlaskConical,
  LineChart,
  ListOrdered,
  RefreshCw,
  Rocket,
} from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { DataTable, SelTeks } from "@/components/data-table/data-table";
import { EmptyState } from "@/components/garda/empty-state";
import {
  HalamanPas,
  IsiPas,
  PanelPas,
  RingkasanKeputusan,
  type ItemRingkas,
} from "@/components/garda/halaman-pas";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { PageHeader } from "@/components/garda/page-header";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { SpringButton } from "@/components/garda/spring-button";
import { StatusBadge } from "@/components/garda/status-badge";
import { ViewTabs } from "@/components/garda/view-tabs";
import { CountUp } from "@/components/motion/count-up";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  alasanTakBisaSegarkan,
  modelUsang,
  ringkasEvaluasi,
  ringkasSegarkan,
  type BarisModel,
  type BarisProyeksi,
} from "@/lib/api/prediksi";
import { useCan } from "@/lib/auth/session-context";
import { exportCsv } from "@/lib/csv";
import { KOMODITAS } from "@/lib/data/reference";
import { formatPersen, formatRelatif, formatRupiah } from "@/lib/format";
import { GLOSARIUM } from "@/lib/glosarium";
import { useTabHalaman } from "@/lib/hooks/use-tab-halaman";
import {
  useAksiModel,
  useEvaluasiPrediksi,
  useLatihModel,
  useModelPrediksi,
  useProyeksiRingkasan,
  useSegarkanProyeksi,
  useSimulasi,
  useStatusMl,
  useXai,
} from "@/lib/query/hooks";
import { cn } from "@/lib/utils";
import { PemilihSiklusMl } from "@/components/ml/siklus-ml";
import { namaKomoditasMl } from "@/lib/ml/label";
import { PanelPenjelasan, PanelProyeksi } from "./panel-proyeksi";
import {
  barisKomoditasMl,
  KABUPATEN_ML_BAWAAN,
  TabModelMl,
  TabProyeksiMl,
  usePrediksiMl,
} from "./prediksi-ml";

const TAB = ["proyeksi", "komoditas", "simulasi", "model"] as const;
const KOMODITAS_PILOT = KOMODITAS.filter((k) => k.pilot);

function Slider({
  label,
  v,
  set,
  min,
  max,
  unit,
}: {
  label: string;
  v: number;
  set: (n: number) => void;
  min: number;
  max: number;
  unit: string;
}) {
  return (
    <label className="block">
      <span className="flex justify-between gap-2 text-[13px]">
        <span>{label}</span>
        <span className="num text-muted-foreground">
          {v}
          {unit}
        </span>
      </span>
      <input
        type="range"
        min={min}
        max={max}
        value={v}
        onChange={(e) => set(Number(e.target.value))}
        className="mt-2 w-full accent-[var(--color-data-600)]"
      />
    </label>
  );
}

/**
 * Simulasi what-if yang dihitung dan disimpan server dari proyeksi model komoditas terpilih.
 * Koefisien sensitivitas masih asumsi (belum diestimasi dari data) dan diberi label tegas.
 */
function SimulasiSkenario({
  kode,
  nama,
  className,
}: {
  kode: string;
  nama: string;
  className?: string;
}) {
  const [hujan, setHujan] = useState(100);
  const [pasokan, setPasokan] = useState(0);
  const [subsidi, setSubsidi] = useState(0);
  const { mutate, data: hasil, isPending, reset } = useSimulasi();
  // Hasil milik komoditas sebelumnya tidak boleh tampak berlaku untuk yang baru dipilih.
  useEffect(() => {
    reset();
  }, [kode, reset]);

  return (
    <IsiPas
      kolom="lg:grid-cols-[minmax(0,24rem)_minmax(0,1fr)]"
      className={className}
    >
      <PanelPas
        judul="Simulasi skenario"
        deskripsi={`${nama}, Banda Aceh, 14 hari`}
      >
        <div className="space-y-4">
          <Slider
            label="Curah hujan sentra (vs normal)"
            v={hujan}
            set={setHujan}
            min={50}
            max={250}
            unit="%"
          />
          <Slider
            label="Tambahan pasokan antardaerah"
            v={pasokan}
            set={setPasokan}
            min={0}
            max={40}
            unit="%"
          />
          <Slider
            label="Subsidi ongkos angkut"
            v={subsidi}
            set={setSubsidi}
            min={0}
            max={30}
            unit="%"
          />
          <SpringButton
            disabled={isPending}
            onClick={() =>
              mutate({
                komoditasKode: kode,
                hujanPct: hujan,
                pasokanPct: pasokan,
                subsidiPct: subsidi,
              })
            }
          >
            {isPending ? "Menghitung" : "Hitung simulasi"}
          </SpringButton>
        </div>
      </PanelPas>
      <PanelPas
        judul="Hasil simulasi"
        deskripsi="Perkiraan harga bila skenario terjadi"
      >
        {hasil ? (
          <div
            className={cn(
              "rounded-lg p-4",
              hasil.hasilPct > 5
                ? "bg-defisit-soft/70 dark:bg-defisit/15"
                : hasil.hasilPct < -3
                  ? "bg-surplus-soft/70 dark:bg-surplus/15"
                  : "bg-normal-soft/70 dark:bg-normal/15",
            )}
          >
            <p className="text-xs text-muted-foreground">
              Proyeksi perubahan harga (model{" "}
              {formatPersen(hasil.dasarPct, { signed: true })})
            </p>
            <p className="mt-1 text-3xl font-semibold">
              <CountUp
                value={hasil.hasilPct}
                onView={false}
                duration={0.5}
                format={(n) => formatPersen(n, { signed: true })}
              />
            </p>
            <p className="num mt-1 text-xs text-muted-foreground">
              Rentang {formatPersen(hasil.rentang[0], { signed: true })} s.d.{" "}
              {formatPersen(hasil.rentang[1], { signed: true })} ·{" "}
              {formatRupiah(hasil.proyeksi)}
            </p>
            <ul className="mt-3 space-y-0.5 text-xs text-muted-foreground">
              {hasil.efek.map((e) => (
                <li key={e.faktor} className="flex justify-between gap-2">
                  <span>
                    {e.faktor} ({e.sumber.toLowerCase()})
                  </span>
                  <span className="num">
                    {formatPersen(e.efekPct, { signed: true })}
                  </span>
                </li>
              ))}
            </ul>
            <p role="note" className="mt-3 text-xs text-muted-foreground">
              {hasil.catatan}
            </p>
          </div>
        ) : (
          <EmptyState
            icon={FlaskConical}
            title="Belum ada simulasi"
            description="Atur skenario di kiri, lalu tekan Hitung simulasi."
            className="py-10"
          />
        )}
      </PanelPas>
    </IsiPas>
  );
}

export function PrediksiPage() {
  const params = useSearchParams();
  const router = useRouter();
  const [tab, setTab] = useTabHalaman(TAB, "proyeksi");
  const [kode, setKode] = useState(params.get("komoditas") ?? "CBM-KRT");
  const {
    data: xai,
    isLoading: memuatXai,
    isFetching: menyegarkanXai,
    isError: xaiGalat,
    error: galatXai,
    refetch: muatUlangXai,
  } = useXai(kode);
  const { data: proyeksiLama, isLoading: memuatProyeksiLama } =
    useProyeksiRingkasan();
  const { data: model, isLoading: memuatModel } = useModelPrediksi();
  // Mode Prod dengan hasil training: proyeksi, tabel komoditas, dan metrik model mengikuti siklus pilihan (?cycle_id=).
  const [kabupaten, setKabupaten] = useState(
    params.get("kabupaten") ?? KABUPATEN_ML_BAWAAN,
  );
  const ml = usePrediksiMl(kode, kabupaten);
  const barisMl = ml.aktif ? barisKomoditasMl(ml) : undefined;
  const proyeksi = ml.aktif ? barisMl : proyeksiLama;
  const memuatProyeksi = ml.aktif ? barisMl === undefined : memuatProyeksiLama;
  const bisaKelola = useCan("model.kelola");
  const { data: statusMl } = useStatusMl();
  const latih = useLatihModel();
  const aksiModel = useAksiModel();
  const segarkan = useSegarkanProyeksi();
  const evaluasi = useEvaluasiPrediksi();
  const resetSegarkan = segarkan.reset;
  // Hasil penyegaran milik komoditas sebelumnya tidak boleh tampak berlaku untuk yang baru dipilih.
  useEffect(() => {
    resetSegarkan();
  }, [kode, resetSegarkan]);
  const barisTerpilih = proyeksi?.find((p) => p.id === kode);
  const namaTerpilih =
    barisTerpilih?.nama ?? (ml.aktif ? namaKomoditasMl(kode) : kode);
  const demo = [...(proyeksi ?? []), ...(model ?? [])].some((r) => r.demo);
  const alasanSegarkan = alasanTakBisaSegarkan(
    model,
    kode,
    !!statusMl?.tersedia,
  );
  const catatanSegarkan =
    bisaKelola && alasanSegarkan
      ? `Segarkan proyeksi ${namaTerpilih} belum bisa: ${alasanSegarkan[0].toLowerCase()}${alasanSegarkan.slice(1)}`
      : null;

  const opsiKomoditas = useMemo(() => {
    const daftar = proyeksi?.length
      ? proyeksi.map((p) => ({ kode: p.id, nama: p.nama }))
      : KOMODITAS_PILOT.map((k) => ({ kode: k.kode, nama: k.nama }));
    return daftar.some((d) => d.kode === kode)
      ? daftar
      : [{ kode, nama: kode }, ...daftar];
  }, [proyeksi, kode]);

  const perluTindakan = useMemo(
    () => (proyeksi ?? []).filter((p) => p.pct > 5).length,
    [proyeksi],
  );

  const ringkasan: ItemRingkas[] = [
    {
      label: `Proyeksi ${namaTerpilih}`,
      nilai: xai ? formatPersen(xai.perubahanPct, { signed: true }) : "–",
      keterangan: xai ? `dalam ${xai.horizonHari} hari` : "Memuat…",
      tone: !xai
        ? "netral"
        : xai.perubahanPct > 5
          ? "defisit"
          : xai.perubahanPct < -3
            ? "surplus"
            : "normal",
      arti: GLOSARIUM.delta,
    },
    {
      label: "Harga kini",
      nilai: xai ? formatRupiah(xai.hargaKini) : "–",
      keterangan: barisTerpilih
        ? `Menjadi ${formatRupiah(barisTerpilih.proyeksi)}`
        : "per kg",
      tone: "data",
    },
    {
      label: "Perlu tindakan",
      nilai: memuatProyeksi ? "–" : `${perluTindakan} komoditas`,
      keterangan: "Diproyeksi naik di atas 5%",
      tone: perluTindakan > 0 ? "defisit" : "normal",
      arti: "Buka daftar semua komoditas, urut dari kenaikan terbesar.",
      onClick: () => setTab("komoditas"),
    },
    {
      label: "Akurasi model",
      nilai: xai ? formatPersen(xai.model.mape) : "–",
      keterangan: "Rata-rata meleset",
      tone: "netral",
      arti: GLOSARIUM.mape,
      onClick: () => setTab("model"),
    },
  ];

  const kolomProyeksi = useMemo<ColumnDef<BarisProyeksi>[]>(
    () => [
      {
        accessorKey: "nama",
        header: "Komoditas",
        cell: ({ row: { original: r } }) => (
          <button
            type="button"
            onClick={() => {
              setKode(r.id);
              setTab("proyeksi");
            }}
            className="flex max-w-full min-w-0 items-center gap-2 text-left font-medium hover:text-data-700"
          >
            <KomoditasIcon
              kode={r.id}
              className="shrink-0 text-muted-foreground"
            />
            <SelTeks>{r.nama}</SelTeks>
          </button>
        ),
      },
      {
        accessorKey: "pct",
        header: "Perubahan",
        cell: ({ getValue }) => {
          const v = getValue<number>();
          return (
            <span
              className={cn(
                "num font-semibold",
                v > 5
                  ? "text-defisit-ink"
                  : v < -3
                    ? "text-surplus-ink"
                    : "text-normal-ink",
              )}
            >
              {formatPersen(v, { signed: true })}
            </span>
          );
        },
      },
      {
        accessorKey: "kini",
        header: "Harga kini",
        cell: ({ getValue }) => (
          <span className="num">{formatRupiah(getValue<number>())}</span>
        ),
      },
      {
        accessorKey: "proyeksi",
        header: "Proyeksi 14 hari",
        cell: ({ getValue }) => (
          <span className="num font-semibold">
            {formatRupiah(getValue<number>())}
          </span>
        ),
      },
      {
        id: "interval",
        header: "Rentang 80%",
        cell: ({ row: { original: r } }) => (
          <span className="num text-xs text-muted-foreground">
            {formatPersen(r.lo, { signed: true })} s.d.{" "}
            {formatPersen(r.hi, { signed: true })}
          </span>
        ),
      },
      {
        accessorKey: "mape",
        header: "MAPE",
        cell: ({ getValue }) => (
          <span className="num">{formatPersen(getValue<number>())}</span>
        ),
      },
    ],
    [setTab],
  );

  const kolomModel = useMemo<ColumnDef<BarisModel>[]>(
    () => [
      {
        accessorKey: "komoditas",
        header: "Komoditas",
        cell: ({ row: { original: r } }) => (
          <span className="block min-w-0">
            <SelTeks className="max-w-[13rem] font-medium">
              {r.komoditas}
            </SelTeks>
            <span className="num block text-xs whitespace-nowrap text-muted-foreground">
              {r.versi}
            </span>
          </span>
        ),
      },
      {
        accessorKey: "status",
        header: "Status",
        filterFn: "equals",
        cell: ({ getValue }) => {
          const s = getValue<BarisModel["status"]>();
          return (
            <StatusBadge
              tone={
                s === "PRODUKSI"
                  ? "normal"
                  : s === "KANDIDAT"
                    ? "data"
                    : "netral"
              }
            >
              {s === "PRODUKSI"
                ? "Produksi"
                : s === "KANDIDAT"
                  ? "Kandidat"
                  : "Arsip"}
            </StatusBadge>
          );
        },
      },
      { accessorKey: "versi", header: "Versi" },
      {
        accessorKey: "mape",
        header: "MAPE backtest",
        cell: ({ getValue }) => (
          <span className="num">{formatPersen(getValue<number>())}</span>
        ),
      },
      {
        accessorKey: "baselineMape",
        header: "Baseline naif",
        cell: ({ getValue }) => {
          const v = getValue<number | null>();
          return (
            <span className="num text-muted-foreground">
              {v === null ? "–" : formatPersen(v)}
            </span>
          );
        },
      },
      {
        accessorKey: "mapeRealisasi",
        header: "MAPE realisasi",
        cell: ({ row: { original: r } }) => (
          <span className="num text-muted-foreground">
            {r.mapeRealisasi === null
              ? "–"
              : `${formatPersen(r.mapeRealisasi)} (${r.nEvaluasi})`}
          </span>
        ),
      },
      {
        accessorKey: "dilatihPada",
        header: "Dilatih",
        cell: ({ row: { original: r } }) => (
          <span className="flex items-center gap-1.5 whitespace-nowrap text-muted-foreground">
            <span className="num">{formatRelatif(r.dilatihPada)}</span>
            {modelUsang(r) && <StatusBadge tone="waspada">Usang</StatusBadge>}
            {r.demo && <StatusBadge tone="netral">Demo</StatusBadge>}
          </span>
        ),
      },
      {
        accessorKey: "drift",
        header: "Drift (PSI)",
        cell: ({ getValue }) => {
          const v = getValue<number>();
          return (
            <span
              className={cn(
                "num",
                v > 0.1 ? "text-waspada-ink" : "text-muted-foreground",
              )}
            >
              {v.toFixed(2).replace(".", ",")}
            </span>
          );
        },
      },
    ],
    [],
  );

  const tombolModel = bisaKelola ? (
    <>
      <SpringButton
        variant="soft"
        size="sm"
        title={alasanSegarkan ?? undefined}
        disabled={segarkan.isPending || alasanSegarkan !== null}
        onClick={() => segarkan.mutate(kode)}
      >
        <RefreshCw className={cn(segarkan.isPending && "animate-spin")} />{" "}
        Segarkan proyeksi
      </SpringButton>
      <SpringButton
        variant="soft"
        size="sm"
        disabled={evaluasi.isPending}
        onClick={() => evaluasi.mutate()}
      >
        <ClipboardCheck /> Evaluasi akurasi
      </SpringButton>
    </>
  ) : undefined;

  const adaHasil =
    segarkan.data || segarkan.isError || evaluasi.data || evaluasi.isError;

  return (
    <HalamanPas>
      <PageHeader
        title="Prediksi harga"
        description="Arah harga 14 hari dan alasannya."
        tabs={
          <ViewTabs
            layoutId="tab-prediksi"
            label="Tampilan prediksi"
            value={tab}
            onChange={setTab}
            tabs={[
              { value: "proyeksi", label: "Proyeksi", icon: <LineChart /> },
              {
                value: "komoditas",
                label: "Komoditas",
                icon: <ListOrdered />,
                count: proyeksi?.length,
              },
              { value: "simulasi", label: "Simulasi", icon: <FlaskConical /> },
              {
                value: "model",
                label: "Model",
                icon: <Cpu />,
                count: model?.length,
              },
            ]}
          />
        }
        actions={
          <>
            <Select value={kode} onValueChange={setKode}>
              <SelectTrigger
                aria-label="Pilih komoditas"
                className="w-full text-[13px] sm:w-52 md:text-[13px] lg:w-40 min-[88rem]:w-52"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {opsiKomoditas.map((k) => (
                  <SelectItem key={k.kode} value={k.kode}>
                    {k.nama}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <PemilihSiklusMl />
          </>
        }
      />

      {demo && (
        <p
          role="note"
          className="shrink-0 rounded-lg border border-waspada/30 bg-waspada-soft/60 px-3 py-1.5 text-xs text-waspada-ink dark:bg-waspada/10"
        >
          Data demo: proyeksi dan registri model di halaman ini berasal dari
          seed contoh, bukan keluaran model sungguhan.
        </p>
      )}

      {adaHasil && (
        <div className="shrink-0 space-y-1.5 text-[13px]">
          {segarkan.data && (
            <p
              role="status"
              className="rounded-lg bg-normal-soft/70 px-3 py-1.5 dark:bg-normal/15"
            >
              Hasil penyegaran: {ringkasSegarkan(segarkan.data, namaTerpilih)}
            </p>
          )}
          {segarkan.isError && (
            <p
              role="alert"
              className="rounded-lg bg-defisit-soft/70 px-3 py-1.5 text-defisit-ink dark:bg-defisit/15"
            >
              Penyegaran gagal: {segarkan.error.message}
            </p>
          )}
          {evaluasi.data && (
            <p
              role="status"
              className="rounded-lg bg-normal-soft/70 px-3 py-1.5 dark:bg-normal/15"
            >
              Hasil evaluasi: {ringkasEvaluasi(evaluasi.data)}
            </p>
          )}
          {evaluasi.isError && (
            <p
              role="alert"
              className="rounded-lg bg-defisit-soft/70 px-3 py-1.5 text-defisit-ink dark:bg-defisit/15"
            >
              Evaluasi gagal: {evaluasi.error.message}
            </p>
          )}
        </div>
      )}

      {tab === "proyeksi" && ml.aktif && (
        <TabProyeksiMl
          ml={ml}
          kode={kode}
          kabupaten={kabupaten}
          onKabupaten={setKabupaten}
          onKomoditas={setKode}
          bukaKomoditas={() => setTab("komoditas")}
          bukaModel={() => setTab("model")}
        />
      )}

      {tab === "proyeksi" && !ml.aktif && (
        <>
          <RingkasanKeputusan items={ringkasan} label="Ringkasan proyeksi" />
          <IsiPas kolom="lg:grid-cols-[minmax(0,1fr)_22rem]">
            {xai ? (
              <>
                <PanelProyeksi
                  data={xai}
                  memuatUlang={menyegarkanXai}
                  aksiModel={tombolModel}
                  catatan={catatanSegarkan}
                />
                <PanelPenjelasan data={xai} />
              </>
            ) : xaiGalat ? (
              <PanelGalat
                judul="Proyeksi gagal dimuat."
                error={galatXai}
                onCoba={() => void muatUlangXai()}
                className="lg:col-span-2"
              />
            ) : (
              <>
                <Shimmer
                  aria-busy={memuatXai}
                  className="h-80 w-full rounded-lg lg:h-full"
                />
                <Shimmer className="h-64 w-full rounded-lg lg:h-full" />
              </>
            )}
          </IsiPas>
        </>
      )}

      {tab === "komoditas" && (
        <IsiPas>
          <p className="shrink-0 text-[13px] text-muted-foreground">
            Klik nama komoditas untuk melihat proyeksinya. Centang beberapa
            untuk diteruskan ke kanban intervensi.
          </p>
          <DataTable
            isiPenuh
            data={proyeksi}
            isLoading={memuatProyeksi}
            columns={kolomProyeksi}
            getRowId={(r) => r.id}
            entityLabel="komoditas"
            pageSize={25}
            initialSorting={[{ id: "pct", desc: true }]}
            searchPlaceholder="Cari komoditas"
            rowTone={(r) =>
              r.pct > 5 ? "defisit" : r.pct < -3 ? "surplus" : undefined
            }
            bulkActions={[
              {
                id: "teruskan",
                label: "Teruskan ke intervensi",
                icon: ArrowRightLeft,
                perm: "intervensi.kelola",
                tone: "data",
                run: (r) => {
                  toast.success(`${r.length} kartu intervensi dibuat`);
                  router.push("/intervensi");
                },
              },
              {
                id: "ekspor",
                label: "Ekspor CSV",
                icon: Download,
                run: (r) =>
                  exportCsv(
                    r,
                    [
                      { header: "Komoditas", value: (x) => x.nama },
                      { header: "Kini", value: (x) => x.kini },
                      { header: "Proyeksi", value: (x) => x.proyeksi },
                      { header: "Perubahan (%)", value: (x) => x.pct },
                    ],
                    "proyeksi",
                  ),
              },
            ]}
          />
        </IsiPas>
      )}

      {/* Tetap terpasang agar isian dan hasil simulasi tidak hilang saat berpindah tab. */}
      <SimulasiSkenario
        kode={kode}
        nama={namaTerpilih}
        className={cn(tab !== "simulasi" && "hidden lg:hidden")}
      />

      {tab === "model" && (
        <IsiPas>
          {bisaKelola && (
            <div className="flex shrink-0 flex-col gap-2 text-[13px]">
              <div className="flex flex-wrap items-center gap-2">
                <SpringButton
                  size="sm"
                  disabled={latih.isPending || !statusMl?.tersedia}
                  onClick={() => latih.mutate(kode)}
                >
                  <Cpu /> Latih model {namaTerpilih}
                </SpringButton>
                {tombolModel}
              </div>
              <p role="status" className="text-xs text-muted-foreground">
                {statusMl?.tersedia
                  ? `Layanan ML ${statusMl.versi} siap. `
                  : "Layanan ML tidak tersedia; proyeksi demo tetap tampil. "}
                Model baru berstatus kandidat sampai dipromosikan di tabel.{" "}
                {catatanSegarkan ??
                  "Segarkan meramal ulang dari harga terbaru tanpa melatih ulang."}
              </p>
            </div>
          )}
          {ml.aktif && <TabModelMl ml={ml} kabupaten={kabupaten} />}
          {!ml.aktif && (
            <p className="shrink-0 text-xs text-muted-foreground">
              MAPE backtest bukan jaminan akurasi ke depan. MAPE realisasi
              dihitung dari proyeksi yang sudah jatuh tempo (jumlahnya dalam
              kurung).
            </p>
          )}
          {!ml.aktif && (
            <DataTable
              isiPenuh
              data={model}
              isLoading={memuatModel}
              columns={kolomModel}
              getRowId={(r) => r.id}
              entityLabel="model"
              pageSize={25}
              hiddenColumns={["versi"]}
              searchPlaceholder="Cari model"
              facets={[
                {
                  columnId: "status",
                  label: "Status",
                  options: [
                    { value: "PRODUKSI", label: "Produksi" },
                    { value: "KANDIDAT", label: "Kandidat" },
                    { value: "ARSIP", label: "Arsip" },
                  ],
                },
              ]}
              bulkActions={[
                {
                  id: "promosi",
                  label: "Promosikan ke produksi",
                  icon: Rocket,
                  perm: "model.kelola",
                  tone: "data",
                  enabled: (r) => r.every((x) => x.status !== "PRODUKSI"),
                  confirm: {
                    title: "Promosikan model terpilih?",
                    description: (n) =>
                      `${n} model menggantikan versi produksi komoditasnya; versi lama diarsipkan dan dapat dipromosikan kembali (rollback). Model hasil pelatihan yang MAPE-nya lebih buruk dari baseline ditolak.`,
                    actionLabel: "Promosikan",
                  },
                  run: (r) =>
                    aksiModel.mutateAsync({
                      aksi: "promosi",
                      ids: r.map((x) => x.id),
                      label: "Model dipromosikan",
                    }),
                },
                {
                  id: "arsip",
                  label: "Arsipkan kandidat",
                  icon: Archive,
                  perm: "model.kelola",
                  enabled: (r) => r.every((x) => x.status === "KANDIDAT"),
                  run: (r) =>
                    aksiModel.mutateAsync({
                      aksi: "arsip",
                      ids: r.map((x) => x.id),
                      label: "Model diarsipkan",
                    }),
                },
              ]}
            />
          )}
        </IsiPas>
      )}
    </HalamanPas>
  );
}
