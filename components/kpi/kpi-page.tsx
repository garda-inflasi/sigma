"use client";

import { motion } from "motion/react";
import { Building2, ChartNoAxesCombined, Database, Truck } from "lucide-react";
import { PageHeader } from "@/components/garda/page-header";
import { CountUp } from "@/components/motion/count-up";
import { Sparkline } from "@/components/motion/sparkline";
import { garda } from "@/lib/design/tokens";
import { ease, fadeUp, stagger } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

interface Kpi { nama: string; nilai: number; target: number; satuan: string; naikBaik: boolean; tren: number[] }
const GRUP: { judul: string; icon: typeof Database; kpi: Kpi[] }[] = [
  {
    judul: "Data & sistem", icon: Database, kpi: [
      { nama: "Akurasi proyeksi produksi", nilai: 93.2, target: 98, satuan: "%", naikBaik: true, tren: [88, 89, 90.5, 91, 92.4, 93.2] },
      { nama: "Produsen terverifikasi", nilai: 1284, target: 2000, satuan: "", naikBaik: true, tren: [420, 610, 820, 990, 1150, 1284] },
      { nama: "Kelengkapan entri harian", nilai: 91.5, target: 95, satuan: "%", naikBaik: true, tren: [84, 86, 88, 89, 90.8, 91.5] },
    ]
  },
  {
    judul: "Logistik & infrastruktur", icon: Truck, kpi: [
      { nama: "Utilisasi cold storage", nilai: 71.8, target: 80, satuan: "%", naikBaik: true, tren: [52, 58, 63, 69, 75, 71.8] },
      { nama: "Food loss pascapanen", nilai: 14.6, target: 10, satuan: "%", naikBaik: false, tren: [22, 20.4, 18.8, 17, 15.5, 14.6] },
      { nama: "Volume teragregasi (ton/bln)", nilai: 2140, target: 3000, satuan: "", naikBaik: true, tren: [800, 1100, 1450, 1700, 1980, 2140] },
    ]
  },
  {
    judul: "Dampak ekonomi", icon: ChartNoAxesCombined, kpi: [
      { nama: "Volatilitas harga produsen (CV)", nilai: 8.4, target: 6, satuan: "%", naikBaik: false, tren: [12.1, 11.2, 10.5, 9.6, 9.0, 8.4] },
      { nama: "Margin produsen–konsumen", nilai: 31, target: 25, satuan: "%", naikBaik: false, tren: [39, 37, 35, 34, 32, 31] },
      { nama: "Efisiensi biaya rute", nilai: 12.3, target: 15, satuan: "%", naikBaik: true, tren: [2, 4.5, 6.8, 8.9, 10.7, 12.3] },
    ]
  },
  {
    judul: "Kelembagaan", icon: Building2, kpi: [
      { nama: "Rekomendasi AI ditindaklanjuti", nilai: 68, target: 75, satuan: "%", naikBaik: true, tren: [41, 48, 55, 60, 64, 68] },
      { nama: "Indeks koordinasi lintas lembaga", nilai: 4.1, target: 4.5, satuan: "/5", naikBaik: true, tren: [3.2, 3.4, 3.6, 3.8, 3.9, 4.1] },
    ]
  },
];

/** Ekosistem KPI sesuai blueprint: capaian terhadap target dari hulu ke hilir. */
export function KpiPage() {
  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <PageHeader title="Indikator kinerja" description="Capaian terhadap target fase 1–3. Untuk indikator bertanda turun, nilai lebih rendah berarti lebih baik." />
      <motion.div variants={stagger(0.08)} initial="hidden" animate="show" className="grid gap-6 lg:grid-cols-2">
        {GRUP.map((g) => (
          <motion.section key={g.judul} variants={fadeUp} className="glass rounded-xl p-5 sm:p-6">
            <h2 className="flex items-center gap-2.5 text-display-sm font-semibold"><span className="grid size-9 place-items-center rounded-xl bg-navy-50 text-navy-700 dark:bg-white/10 dark:text-navy-200"><g.icon className="size-[18px]" /></span>{g.judul}</h2>
            <ul className="mt-5 space-y-5">
              {g.kpi.map((k) => {
                const capai = k.naikBaik ? k.nilai / k.target : k.target / k.nilai;
                const pct = Math.min(100, capai * 100);
                const fmt = (n: number) => (k.nilai % 1 ? n.toLocaleString("id-ID", { minimumFractionDigits: 1, maximumFractionDigits: 1 }) : Math.round(n).toLocaleString("id-ID")) + k.satuan;
                return (
                  <li key={k.nama} className="grid grid-cols-[1fr_auto] items-center gap-x-4 gap-y-2">
                    <div className="min-w-0">
                      <p className="truncate text-sm">{k.nama}</p>
                      <p className="mt-0.5 text-xs text-muted-foreground">Target <span className="num">{fmt(k.target)}</span>{k.naikBaik ? "" : ", makin rendah makin baik"}</p>
                    </div>
                    <div className="flex items-center gap-3">
                      <Sparkline data={k.tren} color={capai >= 0.9 ? garda.status.normal.base : garda.data[500]} width={72} height={26} fill={false} />
                      <CountUp value={k.nilai} format={fmt} className="w-20 text-right text-lg font-semibold" />
                    </div>
                    <div className="col-span-2 h-1.5 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10">
                      <motion.div className={cn("h-full rounded-full", capai >= 0.9 ? "bg-normal" : capai >= 0.7 ? "bg-data-500" : "bg-waspada")} initial={{ width: 0 }} whileInView={{ width: `${pct}%` }} viewport={{ once: true }} transition={{ duration: 1, ease: ease.garda }} />
                    </div>
                  </li>
                );
              })}
            </ul>
          </motion.section>
        ))}
      </motion.div>
    </div>
  );
}
