import { CloudRain, PackageX, ShieldAlert, TrendingDown, TrendingUp, type LucideIcon } from "lucide-react";
import type { EwsAlert, JenisEws } from "@/lib/data/types";
import { formatAngka, formatPersen } from "@/lib/format";

export const JENIS_EWS: Record<JenisEws, { label: string; icon: LucideIcon }> = {
  LONJAKAN_HARGA: { label: "Lonjakan harga", icon: TrendingUp },
  KELANGKAAN_STOK: { label: "Kelangkaan stok", icon: PackageX },
  ANOMALI_CUACA: { label: "Anomali cuaca", icon: CloudRain },
  OVER_SUPPLY: { label: "Over-supply", icon: TrendingDown },
  ANOMALI_DATA: { label: "Anomali data", icon: ShieldAlert },
};

export function JenisIcon({ jenis, className }: { jenis: JenisEws; className?: string }) {
  const Icon = JENIS_EWS[jenis].icon;
  return <Icon className={className} aria-hidden />;
}

export function formatNilaiEws(a: Pick<EwsAlert, "nilai" | "satuanNilai">) {
  if (a.satuanNilai === "%") return formatPersen(a.nilai, { signed: true });
  return `${formatAngka(a.nilai, a.satuanNilai === "mm" ? 1 : 0)} ${a.satuanNilai}`;
}
