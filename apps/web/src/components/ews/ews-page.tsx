"use client";

import type { ColumnDef } from "@tanstack/react-table";
import {
  BellRing,
  Check,
  ChevronsUp,
  CircleCheckBig,
  Download,
  History,
  Pencil,
  Play,
  SlidersHorizontal,
  X,
} from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import type { BulkAction } from "@/components/data-table/bulk-action-bar";
import { DataTable } from "@/components/data-table/data-table";
import {
  HalamanPas,
  IsiPas,
  KOLOM_PAS,
  PanelPas,
  RingkasanKeputusan,
  type ItemRingkas,
} from "@/components/garda/halaman-pas";
import { PageHeader } from "@/components/garda/page-header";
import { SpringButton } from "@/components/garda/spring-button";
import { STATUS_EWS, StatusBadge } from "@/components/garda/status-badge";
import { ViewTabs } from "@/components/garda/view-tabs";
import { PulseRing } from "@/components/motion/pulse-ring";
import { Button } from "@/components/ui/button";
import type { EwsAturan } from "@/lib/api/ews";
import { cocokRujukanAlert, satuanAmbang } from "@/lib/api/ews";
import { useCan } from "@/lib/auth/session-context";
import { exportCsv } from "@/lib/csv";
import type { EwsAlert, StatusEws } from "@/lib/data/types";
import { severityMeta, toneClass, type Severity } from "@/lib/design/tokens";
import { formatAngka, formatRelatif } from "@/lib/format";
import { useTabHalaman } from "@/lib/hooks/use-tab-halaman";
import { labelKode } from "@/lib/kode";
import { kritisAktif, pesanTutupMassal } from "@/lib/ews/kritis";
import {
  useAksiEws,
  useEvaluasiEws,
  useEwsAturan,
  useEwsList,
} from "@/lib/query/hooks";
import { cn } from "@/lib/utils";
import { AturanDialog } from "./aturan-dialog";
import { EwsDetailSheet } from "./ews-detail-sheet";
import { JENIS_EWS, JenisIcon, formatNilaiEws } from "./ews-meta";
import { NotifikasiPreferensi } from "./notifikasi-preferensi";
import { PanelSinyalPipelineMl } from "./sinyal-pipeline-ml";
import { PemilihSiklusMl } from "@/components/ml/siklus-ml";

const TAB = ["peringatan", "ambang"] as const;
const URUTAN_TINGKAT: Severity[] = ["KRITIS", "SIAGA", "WASPADA", "INFO"];
const ARTI_TINGKAT: Record<Severity, { singkat: string; arti: string }> = {
  KRITIS: {
    singkat: "Putuskan hari ini",
    arti: "Lewat ambang tertinggi. Putuskan dan bertindak hari ini.",
  },
  SIAGA: {
    singkat: "Siapkan tindakan",
    arti: "Lewat ambang siaga. Siapkan tindakan sekarang.",
  },
  WASPADA: {
    singkat: "Pantau perkembangan",
    arti: "Mulai menyimpang. Pantau dan siapkan antisipasi.",
  },
  INFO: {
    singkat: "Untuk diketahui",
    arti: "Catatan saja, belum perlu tindakan.",
  },
};

const KANAL_LABEL: Record<string, string> = {
  DASHBOARD: "Dashboard",
  PUSH: "Push",
  EMAIL: "Email",
  WHATSAPP: "WhatsApp",
};
const KEPALA_AMBANG =
  "sticky top-0 z-10 h-9 border-b border-border bg-secondary px-3 text-[11px] font-medium tracking-wide text-muted-foreground uppercase";
const SEL_AMBANG = "h-11 border-b border-border px-3 text-[13px]";

/** KRITIS aktif selalu di puncak tabel, apa pun urutan yang dipilih pengguna. */
const KOLOM_PRIORITAS = "prioritasKritis";
const URUTAN_KRITIS = [{ id: KOLOM_PRIORITAS, desc: true }];

const tingkatSah = (nilai: string | null): Severity | undefined =>
  URUTAN_TINGKAT.find((s) => s === nilai);

/** Aturan ambang & kanal notifikasi dari basis data; dapat diubah oleh peran dengan `ews.atur_ambang`. */
function AturanAmbang() {
  const { data, isLoading } = useEwsAturan();
  const bisaAtur = useCan("ews.atur_ambang");
  const evaluasi = useEvaluasiEws();
  const [diubah, setDiubah] = useState<EwsAturan | null>(null);
  const aktif = useMemo(() => (data ?? []).filter((a) => a.aktif), [data]);
  const kanal = useMemo(
    () => [...new Set(aktif.flatMap((a) => a.kanal))],
    [aktif],
  );
  return (
    <IsiPas kolom="lg:grid-cols-[minmax(0,1fr)_18rem]">
      <PanelPas
        judul="Ambang peringatan"
        deskripsi="Per komoditas dan jenis. Satuan: Δ% terhadap rata-rata jendela, atau mm hujan."
        aksi={
          bisaAtur && (
            <SpringButton
              variant="soft"
              size="sm"
              disabled={evaluasi.isPending}
              onClick={() => evaluasi.mutate()}
            >
              <Play />{" "}
              {evaluasi.isPending ? "Mengevaluasi…" : "Evaluasi sekarang"}
            </SpringButton>
          )
        }
        tanpaPadding
        isiClassName="overflow-x-auto"
      >
        <table className="w-full min-w-[420px] border-separate border-spacing-0 text-[13px]">
          <thead>
            <tr>
              <th className={cn(KEPALA_AMBANG, "text-left")}>Aturan</th>
              {(["WASPADA", "SIAGA", "KRITIS"] as Severity[]).map((s) => (
                <th
                  key={s}
                  className={cn(
                    KEPALA_AMBANG,
                    "text-right",
                    toneClass[severityMeta[s].tone].text,
                  )}
                >
                  {severityMeta[s].label}
                </th>
              ))}
              {bisaAtur && <th className={cn(KEPALA_AMBANG, "w-10")} />}
            </tr>
          </thead>
          <tbody className="[&>tr:last-child>td]:border-b-0">
            {isLoading && (
              <tr>
                <td
                  colSpan={bisaAtur ? 5 : 4}
                  className={cn(
                    SEL_AMBANG,
                    "text-center text-xs text-muted-foreground",
                  )}
                >
                  Memuat ambang…
                </td>
              </tr>
            )}
            {!isLoading && aktif.length === 0 && (
              <tr>
                <td
                  colSpan={bisaAtur ? 5 : 4}
                  className={cn(
                    SEL_AMBANG,
                    "text-center text-xs text-muted-foreground",
                  )}
                >
                  Belum ada aturan ambang aktif.
                </td>
              </tr>
            )}
            {aktif.map((a) => (
              <tr key={a.id} className="hover:bg-secondary/60">
                <td className={cn(SEL_AMBANG, "py-1.5")}>
                  <span className="block">
                    {a.komoditasNama ?? "Semua komoditas"}
                  </span>
                  <span className="block text-xs text-muted-foreground">
                    {JENIS_EWS[a.jenis].label} · {a.jendelaHari} hari
                  </span>
                </td>
                {[a.waspada, a.siaga, a.kritis].map((v, j) => (
                  <td key={j} className={cn(SEL_AMBANG, "num text-right")}>
                    {formatAngka(v)}
                    {satuanAmbang(a.jenis)}
                  </td>
                ))}
                {bisaAtur && (
                  <td className={cn(SEL_AMBANG, "text-right")}>
                    <button
                      type="button"
                      aria-label={`Ubah ambang ${a.komoditasNama ?? "semua komoditas"} ${JENIS_EWS[a.jenis].label}`}
                      onClick={() => setDiubah(a)}
                      className="rounded-md p-1 text-muted-foreground hover:bg-secondary hover:text-foreground"
                    >
                      <Pencil className="size-3.5" />
                    </button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </PanelPas>
      <PanelPas judul="Cara kerja ambang">
        <div className="space-y-3 text-[13px]">
          <p className="text-muted-foreground">
            {aktif[0]?.keterangan ??
              "Alert dibuat saat nilai melewati ambang aturan yang aktif."}
          </p>
          <div>
            <p className="font-medium">Kanal notifikasi</p>
            {kanal.length === 0 ? (
              <p className="mt-1 text-xs text-muted-foreground">
                Belum ada kanal pada aturan aktif.
              </p>
            ) : (
              <div className="mt-2 flex flex-wrap gap-1.5">
                {kanal.map((k) => (
                  <StatusBadge key={k} tone="netral" dot={false}>
                    {KANAL_LABEL[k] ?? k}
                  </StatusBadge>
                ))}
              </div>
            )}
          </div>
        </div>
      </PanelPas>
      <AturanDialog aturan={diubah} onClose={() => setDiubah(null)} />
    </IsiPas>
  );
}

export function EwsPage() {
  const params = useSearchParams();
  const sev = params.get("severity");
  const sorot = params.get("id");
  const [tab, setTab] = useTabHalaman(TAB, "peringatan");
  const { data, isLoading } = useEwsList();
  const patch = (aksi: string) =>
    (aksi === "akui"
      ? "DIAKUI"
      : aksi === "eskalasi"
        ? "DIESKALASI"
        : "SELESAI") as StatusEws;
  const mut = useAksiEws((r, a) => ({ ...r, status: patch(a) }));
  // Detail yang terbuka ikut tersegarkan setelah aksi (invalidate di atas); ?id= dari feed langsung membukanya.
  const [detailId, setDetailId] = useState<string | null>(sorot);
  useEffect(() => {
    if (sorot) setDetailId(sorot);
  }, [sorot]);

  const [saring, setSaring] = useState(() => tingkatSah(sev));
  useEffect(() => {
    setSaring(tingkatSah(sev));
  }, [sev]);
  const [tampilSelesai, setTampilSelesai] = useState(false);

  const ringkas = useMemo(() => {
    const n: Record<Severity, number> = {
      INFO: 0,
      WASPADA: 0,
      SIAGA: 0,
      KRITIS: 0,
    };
    for (const a of data ?? []) if (a.status !== "SELESAI") n[a.severity]++;
    return n;
  }, [data]);
  const jumlahSelesai = useMemo(
    () => (data ?? []).filter((a) => a.status === "SELESAI").length,
    [data],
  );
  const daftar = useMemo(
    () =>
      data?.filter(
        (a) =>
          (tampilSelesai ||
            a.status !== "SELESAI" ||
            cocokRujukanAlert(a, sorot)) &&
          (!saring || a.severity === saring),
      ),
    [data, tampilSelesai, saring, sorot],
  );

  const ringkasan: ItemRingkas[] = URUTAN_TINGKAT.map((s) => ({
    label: severityMeta[s].label,
    nilai: isLoading ? "–" : `${ringkas[s]} aktif`,
    keterangan: ARTI_TINGKAT[s].singkat,
    tone: severityMeta[s].tone,
    arti: ARTI_TINGKAT[s].arti,
    aktif: saring === s,
    onClick: () => setSaring((v) => (v === s ? undefined : s)),
  }));

  const columns = useMemo<ColumnDef<EwsAlert>[]>(
    () => [
      {
        accessorKey: "severity",
        header: "Tingkat",
        size: 96,
        filterFn: "equals",
        sortingFn: (a, b) =>
          severityMeta[a.original.severity].rank -
          severityMeta[b.original.severity].rank,
        cell: ({ row: { original: a } }) => {
          const m = severityMeta[a.severity];
          return (
            <span className="inline-flex items-center gap-2">
              <PulseRing
                tone={m.tone}
                rings={1}
                active={a.status === "BARU" && m.rank >= 2}
                className="size-2.5"
              >
                <span
                  className={cn(
                    "relative size-2.5 rounded-full",
                    toneClass[m.tone].bg,
                  )}
                />
              </PulseRing>
              <span
                className={cn("text-xs font-medium", toneClass[m.tone].text)}
              >
                {m.label}
              </span>
            </span>
          );
        },
      },
      {
        id: "alert",
        accessorFn: (a) => `${a.judul} ${a.pesan}`,
        header: "Peringatan",
        size: 480,
        // Lebar nol + min-w-full: teks panjang terpotong mengikuti kolom, bukan melebarkan tabel.
        cell: ({ row: { original: a } }) => (
          <div className="flex w-0 min-w-full items-start gap-2.5">
            <span title={JENIS_EWS[a.jenis].label} className="mt-0.5 shrink-0">
              <JenisIcon
                jenis={a.jenis}
                className="size-4 text-muted-foreground"
              />
            </span>
            <div className="min-w-0 flex-1">
              <p
                title={a.judul}
                className={cn(
                  "truncate font-medium",
                  cocokRujukanAlert(a, sorot) &&
                    "text-data-700 dark:text-data-300",
                )}
              >
                {a.judul}
              </p>
              <p
                title={`${a.wilayah} · ${a.pesan}`}
                className="truncate text-xs text-muted-foreground"
              >
                {a.wilayah} · {a.pesan}
              </p>
            </div>
          </div>
        ),
      },
      { accessorKey: "jenis", header: "Jenis", filterFn: "equals" },
      { accessorKey: "wilayah", header: "Wilayah" },
      {
        accessorKey: "nilai",
        header: "Perubahan",
        size: 96,
        cell: ({ row: { original: a } }) => (
          <span
            className={cn(
              "num font-semibold whitespace-nowrap",
              toneClass[severityMeta[a.severity].tone].text,
            )}
          >
            {formatNilaiEws(a)}
          </span>
        ),
      },
      {
        accessorKey: "skorDampak",
        header: "Dampak",
        size: 108,
        cell: ({ getValue }) => {
          const v = getValue<number>();
          return (
            <span className="flex items-center gap-2">
              <span className="h-1.5 w-12 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10">
                <span
                  className={cn(
                    "block h-full rounded-full",
                    v > 75
                      ? "bg-defisit"
                      : v > 50
                        ? "bg-waspada"
                        : "bg-data-500",
                  )}
                  style={{ width: `${v}%` }}
                />
              </span>
              <span className="num text-xs">{v}</span>
            </span>
          );
        },
      },
      {
        accessorKey: "waktu",
        header: "Status",
        size: 124,
        cell: ({ row: { original: a } }) => {
          const st = STATUS_EWS[a.status];
          return (
            <span className="flex flex-col items-start gap-0.5">
              <StatusBadge tone={st.tone} className="h-5 text-[11px]">
                {st.label}
              </StatusBadge>
              <span className="num text-[11px] whitespace-nowrap text-muted-foreground">
                {formatRelatif(a.waktu)}
              </span>
            </span>
          );
        },
      },
      { accessorKey: "status", header: "Status", filterFn: "equals" },
      {
        id: KOLOM_PRIORITAS,
        accessorFn: (a) => (kritisAktif(a) ? 1 : 0),
        header: "Prioritas",
        sortingFn: "basic",
        enableGlobalFilter: false,
      },
    ],
    [sorot],
  );

  const ids = (r: EwsAlert[]) => r.map((x) => x.id);
  const actions: BulkAction<EwsAlert>[] = [
    {
      id: "akui",
      label: "Akui",
      icon: Check,
      perm: "ews.akui",
      tone: "data",
      enabled: (r) => r.every((x) => x.status === "BARU"),
      run: (r) =>
        mut.mutateAsync({ aksi: "akui", ids: ids(r), label: "Alert diakui" }),
    },
    {
      id: "eskalasi",
      label: "Eskalasi",
      icon: ChevronsUp,
      perm: "ews.eskalasi",
      run: (r) =>
        mut.mutateAsync({
          aksi: "eskalasi",
          ids: ids(r),
          label: "Alert dieskalasi ke Dewan Pengarah",
        }),
    },
    {
      id: "selesai",
      label: "Tandai selesai",
      icon: CircleCheckBig,
      perm: "ews.akui",
      confirm: {
        title: "Tutup alert terpilih?",
        description: pesanTutupMassal,
        actionLabel: "Tandai selesai",
      },
      run: (r) =>
        mut.mutateAsync({
          aksi: "selesai",
          ids: ids(r),
          label: "Alert diselesaikan",
        }),
    },
    {
      id: "ekspor",
      label: "Ekspor CSV",
      icon: Download,
      run: (r) =>
        exportCsv(
          r,
          [
            { header: "ID", value: (x) => labelKode(x) },
            { header: "Tingkat", value: (x) => x.severity },
            { header: "Judul", value: (x) => x.judul },
            { header: "Wilayah", value: (x) => x.wilayah },
            { header: "Nilai", value: (x) => x.nilai },
            { header: "Waktu", value: (x) => x.waktu },
            { header: "Status", value: (x) => x.status },
          ],
          "ews",
        ),
    },
  ];

  const opsiStatus = Object.entries(STATUS_EWS)
    .filter(([value]) => tampilSelesai || value !== "SELESAI")
    .map(([value, v]) => ({ value, label: v.label }));

  return (
    <HalamanPas>
      <PageHeader
        title="Peringatan dini"
        description="Lihat alert yang paling berdampak, lalu akui, eskalasi, atau selesaikan."
        actions={tab === "peringatan" ? <PemilihSiklusMl /> : undefined}
        tabs={
          <ViewTabs
            layoutId="tab-ews"
            label="Tampilan peringatan dini"
            value={tab}
            onChange={setTab}
            tabs={[
              {
                value: "peringatan",
                label: "Peringatan",
                icon: <BellRing />,
                count: isLoading
                  ? undefined
                  : (data?.length ?? 0) - jumlahSelesai,
              },
              {
                value: "ambang",
                label: "Ambang",
                icon: <SlidersHorizontal />,
              },
            ]}
          />
        }
      />

      {tab === "peringatan" && (
        <>
          <RingkasanKeputusan
            items={ringkasan}
            label="Ringkasan peringatan aktif per tingkat"
          />
          <IsiPas kolom="lg:grid-cols-[minmax(0,1fr)_17rem]">
            {/* Filter tingkat ada di luar tabel: remount mengembalikan tabel ke halaman pertama. */}
            <DataTable
              key={`${saring ?? "semua"}-${tampilSelesai}`}
              isiPenuh
              className="[&_table]:min-w-[640px]"
              data={daftar}
              isLoading={isLoading}
              columns={columns}
              getRowId={(r) => r.id}
              entityLabel="alert"
              bulkActions={actions}
              hiddenColumns={["jenis", "wilayah", "status", KOLOM_PRIORITAS]}
              facets={[
                { columnId: "status", label: "Status", options: opsiStatus },
                {
                  columnId: "jenis",
                  label: "Jenis",
                  options: Object.entries(JENIS_EWS).map(([value, v]) => ({
                    value,
                    label: v.label,
                  })),
                },
              ]}
              toolbar={
                <>
                  {saring && (
                    <Button
                      variant="secondary"
                      size="sm"
                      className="h-9 text-[13px]"
                      aria-label={`Hapus filter tingkat ${severityMeta[saring].label}`}
                      onClick={() => setSaring(undefined)}
                    >
                      <X /> {severityMeta[saring].label}
                    </Button>
                  )}
                  {jumlahSelesai > 0 && (
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-9 text-[13px]"
                      aria-pressed={tampilSelesai}
                      title="Tampilkan juga alert yang sudah selesai"
                      onClick={() => setTampilSelesai((v) => !v)}
                    >
                      <History />
                      {tampilSelesai
                        ? "Sembunyikan selesai"
                        : `Selesai (${jumlahSelesai})`}
                    </Button>
                  )}
                </>
              }
              initialSorting={[{ id: "skorDampak", desc: true }]}
              urutanTetap={URUTAN_KRITIS}
              searchPlaceholder="Cari peringatan atau wilayah"
              rowTone={(a) =>
                cocokRujukanAlert(a, sorot) ? "data" : undefined
              }
              onRowClick={(a) => setDetailId(a.id)}
              empty={
                daftar?.length === 0
                  ? {
                      title: saring
                        ? `Tidak ada peringatan ${severityMeta[saring].label} yang aktif`
                        : "Tidak ada peringatan aktif",
                      description:
                        "Peringatan baru masuk otomatis saat nilai melewati ambang.",
                      action: saring ? (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setSaring(undefined)}
                        >
                          Lihat semua tingkat
                        </Button>
                      ) : undefined,
                    }
                  : undefined
              }
            />
            <div className={KOLOM_PAS}>
              <PanelSinyalPipelineMl onBukaAlert={setDetailId} />
              <PanelPas judul="Arti tingkat" className="lg:shrink-0">
                <dl className="space-y-2">
                  {URUTAN_TINGKAT.map((s) => (
                    <div key={s} className="flex gap-2">
                      <span
                        aria-hidden
                        className={cn(
                          "mt-1.5 size-1.5 shrink-0 rounded-full",
                          toneClass[severityMeta[s].tone].dot,
                        )}
                      />
                      <div className="min-w-0">
                        <dt className="text-xs font-medium">
                          {severityMeta[s].label}
                        </dt>
                        <dd className="text-xs leading-snug text-muted-foreground">
                          {ARTI_TINGKAT[s].arti}
                        </dd>
                      </div>
                    </div>
                  ))}
                </dl>
              </PanelPas>
              <NotifikasiPreferensi />
            </div>
          </IsiPas>
        </>
      )}

      {tab === "ambang" && <AturanAmbang />}

      <EwsDetailSheet id={detailId} onClose={() => setDetailId(null)} />
    </HalamanPas>
  );
}
