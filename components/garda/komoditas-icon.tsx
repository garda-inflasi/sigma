import { Beef, Box, Carrot, Drumstick, Droplets, Egg, Fish, Flame, Sprout, Wheat, type LucideIcon } from "lucide-react";
import { komoditasByKode, type IkonKomoditas } from "@/lib/data/reference";
import { cn } from "@/lib/utils";

const MAP: Record<IkonKomoditas, LucideIcon> = {
  beras: Wheat, cabai: Flame, bawang: Sprout, telur: Egg, ayam: Drumstick, sapi: Beef, ikan: Fish, minyak: Droplets, gula: Box, sayur: Carrot,
};

export function KomoditasIcon({ kode, className }: { kode: string; className?: string }) {
  const Icon = MAP[komoditasByKode[kode]?.ikon ?? "sayur"];
  return <Icon className={cn("size-4", className)} aria-hidden />;
}
