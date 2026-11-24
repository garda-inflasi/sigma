"use client";

import {
  BadgeCheck,
  BellRing,
  BrainCircuit,
  CheckCheck,
  ClipboardCheck,
  CloudUpload,
  Database,
  FileText,
  Gauge,
  Layers,
  Lock,
  Radar,
  RefreshCw,
  ScanSearch,
  Send,
  ShieldCheck,
  Smartphone,
  SquareKanban,
  Target,
  Workflow,
} from "lucide-react";
import { DiagramAlur, type NodeAlur } from "@/components/flow/flow";
import { HalamanPas, IsiPas, PanelPas } from "@/components/garda/halaman-pas";
import { PageHeader } from "@/components/garda/page-header";
import { ViewTabs } from "@/components/garda/view-tabs";
import { SIKLUS_TAHAP, SIKLUS_URUTAN } from "@/lib/data/siklus";
import { siklusMeta, toneClass } from "@/lib/design/tokens";
import { useTabHalaman } from "@/lib/hooks/use-tab-halaman";
import { cn } from "@/lib/utils";

const ARSITEKTUR: NodeAlur[] = [
  {
    judul: "Layer 1 — Ingesti data",
    icon: Database,
    tone: "data",
    deskripsi:
      "Menghimpun telemetri produsen, statistik, logistik, dan fiskal.",
    butir: [
      "Aplikasi lapangan",
      "BPS · BI · DJPb",
      "BMKG · OpenWeather",
      "Scraper harga",
    ],
    href: "/sumber-data",
  },
  {
    judul: "Layer 2 — Analitik AI",
    icon: BrainCircuit,
    tone: "surplus",
    deskripsi: "Mesin proyeksi harga & pasokan serta peringatan dini.",
    butir: ["Proyeksi harga", "EWS", "Spasial & rute", "Shelf-life"],
    href: "/prediksi",
  },
  {
    judul: "Layer 3 — Dashboard & DSS",
    icon: Gauge,
    tone: "normal",
    deskripsi: "Ruang kendali keputusan berbasis peran.",
    butir: ["Operasional", "Eksekutif", "Policy brief otomatis"],
    href: "/eksekutif",
  },
  {
    judul: "Layer 4 — Intervensi",
    icon: SquareKanban,
    tone: "waspada",
    deskripsi: "Alur kerja yang beradaptasi pada 3 siklus ekonomi.",
    butir: ["Normal", "Tekanan", "Surplus"],
    href: "/intervensi",
  },
];

const DATA: NodeAlur[] = [
  {
    judul: "Sumber",
    icon: CloudUpload,
    tone: "netral",
    butir: ["Lapangan", "PIHPS", "BPS", "DJPb", "BMKG", "Scraper"],
    metrik: "8 sumber",
  },
  {
    judul: "Bronze",
    icon: Layers,
    tone: "waspada",
    deskripsi:
      "Rekaman mentah apa adanya, diberi ID lineage dan stempel waktu ISO 8601.",
    metrik: "≈48 rb/hari",
  },
  {
    judul: "Silver",
    icon: ScanSearch,
    tone: "data",
    deskripsi:
      "Dibersihkan, dedup, satuan & kode BPS dinormalisasi, diperkaya cuaca.",
    metrik: "97,2% lengkap",
  },
  {
    judul: "Gold",
    icon: BadgeCheck,
    tone: "normal",
    deskripsi: "Agregat wilayah/waktu, fitur model, hanya data terpublikasi.",
    metrik: "deret waktu",
  },
  {
    judul: "Konsumen",
    icon: Radar,
    tone: "surplus",
    butir: ["Model ML", "EWS", "Dashboard", "Policy brief"],
  },
];

const ENTRI: NodeAlur[] = [
  {
    judul: "Catat di pasar",
    icon: Smartphone,
    tone: "data",
    deskripsi: "Form per komoditas, foto, GPS, kode etik.",
    peran: "Agen mahasiswa",
  },
  {
    judul: "Antrean perangkat",
    icon: Database,
    tone: "netral",
    deskripsi: "Tersimpan di IndexedDB saat offline.",
    peran: "Perangkat",
  },
  {
    judul: "Sinkronisasi",
    icon: RefreshCw,
    tone: "data",
    deskripsi: "Idempoten; bentrok jadi konflik untuk diputuskan petugas.",
    peran: "Sync engine",
  },
  {
    judul: "Cek otomatis",
    icon: ShieldCheck,
    tone: "waspada",
    deskripsi: "Rentang ±25%, outlier, geofence, duplikasi.",
    peran: "Sistem",
  },
  {
    judul: "Validasi",
    icon: ClipboardCheck,
    tone: "surplus",
    deskripsi: "Validator bukan penginput (pemisahan tugas).",
    peran: "Koordinator lapangan",
  },
  {
    judul: "Publikasi",
    icon: Send,
    tone: "normal",
    deskripsi: "Masuk gold layer dan dipakai model.",
    peran: "Unit Risiko / Data",
  },
];

const KEPUTUSAN: NodeAlur[] = [
  {
    judul: "Deteksi",
    icon: Radar,
    tone: "data",
    deskripsi: "Ambang dinamis & deteksi anomali real-time.",
    peran: "Model EWS",
  },
  {
    judul: "Alert",
    icon: BellRing,
    tone: "defisit",
    deskripsi: "Toast berdenyut, push, email, WhatsApp.",
    peran: "Sistem",
  },
  {
    judul: "Akui & verifikasi",
    icon: CheckCheck,
    tone: "waspada",
    deskripsi: "Konfirmasi dengan data BI/BPS.",
    peran: "Unit Data · BI · BPS",
  },
  {
    judul: "Kartu intervensi",
    icon: SquareKanban,
    tone: "surplus",
    deskripsi: "Rekomendasi AI masuk kanban siklus.",
    peran: "Manajer Program",
  },
  {
    judul: "Persetujuan",
    icon: Lock,
    tone: "netral",
    deskripsi: "Tahap terkunci izin; fiskal oleh DJPb.",
    peran: "Dewan Pengarah",
  },
  {
    judul: "Evaluasi",
    icon: Target,
    tone: "normal",
    deskripsi: "Dampak diukur di KPI & policy brief.",
    peran: "Semua lapis",
  },
];

const TAB = ["arsitektur", "pipeline", "entri", "siklus", "keputusan"] as const;

const DIAGRAM: Record<
  Exclude<(typeof TAB)[number], "siklus">,
  {
    judul: string;
    deskripsi: string;
    nodes: NodeAlur[];
    aliran?: boolean;
    catatan?: string;
  }
> = {
  arsitektur: {
    judul: "Arsitektur 4 lapis",
    deskripsi: "Klik lapis untuk membuka modulnya",
    nodes: ARSITEKTUR,
  },
  pipeline: {
    judul: "Pipeline data",
    deskripsi:
      "Bronze → silver → gold dengan lineage dan audit trail di setiap rekaman",
    nodes: DATA,
    aliran: true,
    catatan:
      "Validasi lintas sumber: harga lapangan dibandingkan dengan PIHPS dan scraper; selisih di atas ambang memicu alert anomali data.",
  },
  entri: {
    judul: "Alur entri harga lapangan",
    deskripsi: "Draf → diajukan → divalidasi → dipublikasi",
    nodes: ENTRI,
    catatan:
      "Entri yang ditolak kembali ke agen dengan permintaan survei ulang dan tercatat di riwayat perangkat.",
  },
  keputusan: {
    judul: "Dari peringatan ke keputusan",
    deskripsi: "Human-in-the-loop di setiap titik keputusan",
    nodes: KEPUTUSAN,
  },
};

/** Halaman alur sistem & data — peta mental platform untuk seluruh peran. */
export function AlurPage() {
  const [tab, setTab] = useTabHalaman(TAB, "arsitektur");
  const diagram = tab === "siklus" ? null : DIAGRAM[tab];

  return (
    <HalamanPas>
      <PageHeader
        title="Alur sistem & data"
        description="Bagaimana data bergerak dari pasar hingga keputusan."
        tabs={
          <ViewTabs
            layoutId="tab-alur"
            label="Pilih alur"
            value={tab}
            onChange={setTab}
            tabs={[
              { value: "arsitektur", label: "Arsitektur" },
              { value: "pipeline", label: "Pipeline data" },
              { value: "entri", label: "Entri harga" },
              { value: "siklus", label: "Siklus intervensi" },
              { value: "keputusan", label: "Keputusan" },
            ]}
          />
        }
      />

      {diagram ? (
        <PanelPas
          key={tab}
          judul={diagram.judul}
          deskripsi={diagram.deskripsi}
          penuh
          isiClassName="lg:flex lg:flex-col"
        >
          <DiagramAlur
            nodes={diagram.nodes}
            aliran={diagram.aliran}
            catatan={diagram.catatan}
            mengisi
            className="w-full lg:my-auto"
          />
        </PanelPas>
      ) : (
        <IsiPas>
          <p className="shrink-0 text-[13px] text-muted-foreground">
            Tiga siklus intervensi. Tahap bergembok membutuhkan izin persetujuan
            tertentu.
          </p>
          <div className="grid grid-cols-1 gap-3 lg:min-h-0 lg:flex-1 lg:grid-cols-3 lg:grid-rows-[minmax(0,1fr)]">
            {SIKLUS_URUTAN.map((s) => {
              const m = siklusMeta[s];
              const t = toneClass[m.tone];
              return (
                <PanelPas
                  key={s}
                  data-siklus={s}
                  judul={
                    <span className={cn("flex items-center gap-2", t.text)}>
                      <Workflow className="size-4 shrink-0" />
                      {m.label}
                    </span>
                  }
                  deskripsi={m.deskripsi}
                >
                  <ol className="space-y-1.5">
                    {SIKLUS_TAHAP[s].map((tahap, i) => (
                      <li
                        key={tahap.id}
                        className="flex items-center gap-2.5 rounded-md bg-secondary px-2.5 py-2 text-[13px]"
                      >
                        <span
                          className="num grid size-5 shrink-0 place-items-center rounded-md text-[11px] font-semibold text-white"
                          style={{
                            background: "var(--siklus)",
                            opacity:
                              0.4 + 0.6 * (i / (SIKLUS_TAHAP[s].length - 1)),
                          }}
                        >
                          {tahap.id.endsWith("selesai") ? "✓" : i + 1}
                        </span>
                        <span className="min-w-0 flex-1">{tahap.judul}</span>
                        {tahap.izinMasuk && (
                          <Lock
                            className="size-3.5 shrink-0 text-muted-foreground"
                            aria-label="Butuh persetujuan"
                          />
                        )}
                      </li>
                    ))}
                  </ol>
                </PanelPas>
              );
            })}
          </div>
        </IsiPas>
      )}

      <p className="flex shrink-0 items-center gap-2 text-xs text-muted-foreground">
        <FileText className="size-3.5" /> Versi teks diagram ini (Mermaid) ada
        di docs/FLOW.md.
      </p>
    </HalamanPas>
  );
}
