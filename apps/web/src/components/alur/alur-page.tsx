"use client";

import {
  BadgeCheck, BellRing, BrainCircuit, CheckCheck, ClipboardCheck, CloudUpload, Database, FileText, Gauge, Layers, Lock, Radar, RefreshCw,
  ScanSearch, Send, ShieldCheck, Smartphone, SquareKanban, Target, Workflow,
} from "lucide-react";
import { DiagramAlur, type NodeAlur } from "@/components/flow/flow";
import { PageHeader, PanelTitle } from "@/components/garda/page-header";
import { SIKLUS_TAHAP, SIKLUS_URUTAN } from "@/lib/data/siklus";
import { siklusMeta, toneClass } from "@/lib/design/tokens";
import { cn } from "@/lib/utils";

const ARSITEKTUR: NodeAlur[] = [
  { judul: "Layer 1 — Ingesti data", icon: Database, tone: "data", deskripsi: "Menghimpun telemetri produsen, statistik, logistik, dan fiskal.", butir: ["Aplikasi lapangan", "BPS · BI · DJPb", "BMKG · OpenWeather", "Scraper harga"], href: "/sumber-data" },
  { judul: "Layer 2 — Analitik AI", icon: BrainCircuit, tone: "surplus", deskripsi: "Mesin proyeksi harga & pasokan serta peringatan dini.", butir: ["Proyeksi harga", "EWS", "Spasial & rute", "Shelf-life"], href: "/prediksi" },
  { judul: "Layer 3 — Dashboard & DSS", icon: Gauge, tone: "normal", deskripsi: "Ruang kendali keputusan berbasis peran.", butir: ["Operasional", "Eksekutif", "Policy brief otomatis"], href: "/eksekutif" },
  { judul: "Layer 4 — Intervensi", icon: SquareKanban, tone: "waspada", deskripsi: "Alur kerja yang beradaptasi pada 3 siklus ekonomi.", butir: ["Normal", "Tekanan", "Surplus"], href: "/intervensi" },
];

const DATA: NodeAlur[] = [
  { judul: "Sumber", icon: CloudUpload, tone: "netral", butir: ["Lapangan", "PIHPS", "BPS", "DJPb", "BMKG", "Scraper"], metrik: "8 sumber" },
  { judul: "Bronze", icon: Layers, tone: "waspada", deskripsi: "Rekaman mentah apa adanya, diberi ID lineage dan stempel waktu ISO 8601.", metrik: "≈48 rb/hari" },
  { judul: "Silver", icon: ScanSearch, tone: "data", deskripsi: "Dibersihkan, dedup, satuan & kode BPS dinormalisasi, diperkaya cuaca.", metrik: "97,2% lengkap" },
  { judul: "Gold", icon: BadgeCheck, tone: "normal", deskripsi: "Agregat wilayah/waktu, fitur model, hanya data terpublikasi.", metrik: "deret waktu" },
  { judul: "Konsumen", icon: Radar, tone: "surplus", butir: ["Model ML", "EWS", "Dashboard", "Policy brief"] },
];

const ENTRI: NodeAlur[] = [
  { judul: "Catat di pasar", icon: Smartphone, tone: "data", deskripsi: "Form per komoditas, foto, GPS, kode etik.", peran: "Agen mahasiswa" },
  { judul: "Antrean perangkat", icon: Database, tone: "netral", deskripsi: "Tersimpan di IndexedDB saat offline.", peran: "Perangkat" },
  { judul: "Sinkronisasi", icon: RefreshCw, tone: "data", deskripsi: "Idempoten; bentrok jadi konflik untuk diputuskan petugas.", peran: "Sync engine" },
  { judul: "Cek otomatis", icon: ShieldCheck, tone: "waspada", deskripsi: "Rentang ±25%, outlier, geofence, duplikasi.", peran: "Sistem" },
  { judul: "Validasi", icon: ClipboardCheck, tone: "surplus", deskripsi: "Validator bukan penginput (pemisahan tugas).", peran: "Koordinator lapangan" },
  { judul: "Publikasi", icon: Send, tone: "normal", deskripsi: "Masuk gold layer dan dipakai model.", peran: "Unit Risiko / Data" },
];

const KEPUTUSAN: NodeAlur[] = [
  { judul: "Deteksi", icon: Radar, tone: "data", deskripsi: "Ambang dinamis & deteksi anomali real-time.", peran: "Model EWS" },
  { judul: "Alert", icon: BellRing, tone: "defisit", deskripsi: "Toast berdenyut, push, email, WhatsApp.", peran: "Sistem" },
  { judul: "Akui & verifikasi", icon: CheckCheck, tone: "waspada", deskripsi: "Konfirmasi dengan data BI/BPS.", peran: "Unit Data · BI · BPS" },
  { judul: "Kartu intervensi", icon: SquareKanban, tone: "surplus", deskripsi: "Rekomendasi AI masuk kanban siklus.", peran: "Manajer Program" },
  { judul: "Persetujuan", icon: Lock, tone: "netral", deskripsi: "Tahap terkunci izin; fiskal oleh DJPb.", peran: "Dewan Pengarah" },
  { judul: "Evaluasi", icon: Target, tone: "normal", deskripsi: "Dampak diukur di KPI & policy brief.", peran: "Semua lapis" },
];

/** Halaman alur sistem & data — peta mental platform untuk seluruh peran. */
export function AlurPage() {
  return (
    <div className="mx-auto max-w-[1600px] space-y-10">
      <PageHeader title="Alur sistem & data" description="Bagaimana data bergerak dari pasar hingga keputusan: arsitektur 4 lapis, pipeline medallion, alur entri harga, siklus intervensi, dan jalur EWS ke keputusan." />

      <section className="space-y-4">
        <PanelTitle title="Arsitektur 4 lapis" description="Klik lapis untuk membuka modulnya" />
        <DiagramAlur nodes={ARSITEKTUR} />
      </section>

      <section className="space-y-4">
        <PanelTitle title="Pipeline data" description="Bronze → silver → gold dengan lineage dan audit trail di setiap rekaman" />
        <DiagramAlur nodes={DATA} aliran catatan="Validasi lintas sumber: harga lapangan dibandingkan dengan PIHPS dan scraper; selisih di atas ambang memicu alert anomali data." />
      </section>

      <section className="space-y-4">
        <PanelTitle title="Alur entri harga lapangan" description="Draf → diajukan → divalidasi → dipublikasi" />
        <DiagramAlur nodes={ENTRI} catatan="Entri yang ditolak kembali ke agen dengan permintaan survei ulang dan tercatat di riwayat perangkat." />
      </section>

      <section className="space-y-4">
        <PanelTitle title="Tiga siklus intervensi" description="Tahap bergembok membutuhkan izin persetujuan tertentu" />
        <div className="grid gap-4 lg:grid-cols-3">
          {SIKLUS_URUTAN.map((s) => {
            const m = siklusMeta[s];
            const t = toneClass[m.tone];
            return (
              <div key={s} data-siklus={s} className="glass rounded-xl p-5">
                <p className={cn("flex items-center gap-2 font-display font-semibold", t.text)}><Workflow className="size-4" />{m.label}</p>
                <p className="mt-1 text-xs text-muted-foreground">{m.deskripsi}</p>
                <ol className="mt-4 space-y-2">
                  {SIKLUS_TAHAP[s].map((tahap, i) => (
                    <li key={tahap.id} className="flex items-center gap-3 rounded-xl bg-white/60 p-2.5 text-sm dark:bg-white/5">
                      <span className="num grid size-6 shrink-0 place-items-center rounded-lg text-[11px] font-semibold text-white" style={{ background: "var(--siklus)", opacity: 0.4 + 0.6 * (i / (SIKLUS_TAHAP[s].length - 1)) }}>{tahap.id.endsWith("selesai") ? "✓" : i + 1}</span>
                      <span className="flex-1">{tahap.judul}</span>
                      {tahap.izinMasuk && <Lock className="size-3.5 text-muted-foreground" aria-label="Butuh persetujuan" />}
                    </li>
                  ))}
                </ol>
              </div>
            );
          })}
        </div>
      </section>

      <section className="space-y-4">
        <PanelTitle title="Dari peringatan ke keputusan" description="Human-in-the-loop di setiap titik keputusan" />
        <DiagramAlur nodes={KEPUTUSAN} />
      </section>

      <p className="flex items-center gap-2 text-xs text-muted-foreground"><FileText className="size-3.5" /> Versi teks diagram ini (Mermaid) ada di docs/FLOW.md.</p>
    </div>
  );
}
