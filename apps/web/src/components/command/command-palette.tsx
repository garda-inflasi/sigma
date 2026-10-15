"use client";

import {
  ClipboardCheck, CornerDownLeft, MapPin, Moon, PlusCircle, RefreshCw, Siren, Sparkles, SquareKanban, Sun, TrendingUp, Zap, type LucideIcon,
} from "lucide-react";
import { useTheme } from "next-themes";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import {
  CommandDialog, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, CommandSeparator, CommandShortcut,
} from "@/components/ui/command";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { useVisibleNav } from "@/components/layout/sidebar";
import { useSession } from "@/lib/auth/session-context";
import { KABUPATEN, KOMODITAS } from "@/lib/data/reference";
import { namaWilayah } from "@/lib/data/generators";
import { can, type Permission } from "@/lib/rbac/permissions";
import { useUi } from "@/lib/stores/ui-store";

interface Aksi {
  id: string;
  label: string;
  icon: LucideIcon;
  perm?: Permission;
  shortcut?: string;
  keywords?: string[];
  run: () => void;
}

/** Palet perintah ⌘K / Ctrl+K — navigasi & aksi cepat, difilter oleh izin peran. */
export function CommandPalette() {
  const open = useUi((s) => s.commandOpen);
  const setOpen = useUi((s) => s.setCommandOpen);
  const router = useRouter();
  const { role } = useSession();
  const { resolvedTheme, setTheme } = useTheme();
  const groups = useVisibleNav();

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const typing = (e.target as HTMLElement)?.closest("input, textarea, [contenteditable=true]");
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((o) => !o);
      } else if (e.key === "/" && !typing) {
        e.preventDefault();
        setOpen(true);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [setOpen]);

  const run = (fn: () => void) => {
    setOpen(false);
    fn();
  };
  const go = (href: string) => () => router.push(href);
  const gelap = resolvedTheme === "dark";

  const aksi: Aksi[] = (
    [
      { id: "input", label: "Input harga baru", icon: PlusCircle, perm: "harga.input", shortcut: "I", keywords: ["entri", "catat"], run: go("/lapangan/input") },
      { id: "validasi", label: "Buka antrean validasi", icon: ClipboardCheck, perm: "harga.validasi", keywords: ["verifikasi"], run: go("/data/harga?status=DIAJUKAN") },
      { id: "brief", label: "Susun draf policy brief bulan ini", icon: Sparkles, perm: "brief.susun", keywords: ["generate", "laporan"], run: go("/policy-brief?susun=1") },
      { id: "tekanan", label: "Buka kanban siklus tekanan harga", icon: SquareKanban, perm: "intervensi.lihat", keywords: ["intervensi", "defisit"], run: go("/intervensi?siklus=tekanan") },
      { id: "kritis", label: "Lihat alert kritis", icon: Siren, perm: "ews.lihat", keywords: ["ews", "darurat"], run: go("/ews?severity=KRITIS") },
      { id: "sinkron", label: "Sinkronkan antrean offline", icon: RefreshCw, perm: "harga.input", keywords: ["sync"], run: () => window.dispatchEvent(new CustomEvent("garda:sinkron")) },
      { id: "simulasi", label: "Simulasikan alert EWS kritis", icon: Zap, perm: "ews.lihat", keywords: ["demo", "uji"], run: () => window.dispatchEvent(new CustomEvent("garda:simulasi-ews")) },
      { id: "tema", label: gelap ? "Beralih ke mode terang" : "Beralih ke mode gelap", icon: gelap ? Sun : Moon, keywords: ["tema", "dark"], run: () => setTheme(gelap ? "light" : "dark") },
    ] satisfies Aksi[]
  ).filter((a) => !a.perm || can(role, a.perm));

  return (
    <CommandDialog open={open} onOpenChange={setOpen} title="Palet perintah" description="Cari halaman, komoditas, wilayah, atau jalankan aksi cepat" className="overflow-hidden rounded-xl sm:max-w-xl">
      <CommandInput placeholder="Cari halaman, komoditas, wilayah, atau perintah…" />
      <CommandList className="scrollbar-thin max-h-[420px]">
        <CommandEmpty>Tidak ada hasil. Coba “cabai”, “Takengon”, atau “validasi”.</CommandEmpty>

        <CommandGroup heading="Aksi cepat">
          {aksi.map((a) => (
            <CommandItem key={a.id} value={`${a.label} ${a.keywords?.join(" ") ?? ""}`} onSelect={() => run(a.run)} className="rounded-xl">
              <a.icon className="size-4 text-data-600" />
              {a.label}
              {a.shortcut && <CommandShortcut>{a.shortcut}</CommandShortcut>}
            </CommandItem>
          ))}
        </CommandGroup>
        <CommandSeparator />

        {groups.map((g) => (
          <CommandGroup key={g.grup} heading={g.grup}>
            {g.items.map((i) => (
              <CommandItem key={i.href} value={`${i.label} ${i.keywords?.join(" ") ?? ""}`} onSelect={() => run(go(i.href))} className="rounded-xl">
                <i.icon className="size-4 text-muted-foreground" />
                {i.label}
              </CommandItem>
            ))}
          </CommandGroup>
        ))}

        {can(role, "prediksi.lihat") && (
          <>
            <CommandSeparator />
            <CommandGroup heading="Komoditas">
              {KOMODITAS.map((k) => (
                <CommandItem key={k.kode} value={`prediksi ${k.nama} ${k.kode} ${k.kategori}`} onSelect={() => run(go(`/prediksi?komoditas=${k.kode}`))} className="rounded-xl">
                  <KomoditasIcon kode={k.kode} className="text-muted-foreground" />
                  Prediksi {k.nama.toLowerCase()}
                  <TrendingUp className="ml-auto size-3.5 text-muted-foreground" />
                </CommandItem>
              ))}
            </CommandGroup>
          </>
        )}

        {can(role, "spasial.lihat") && (
          <>
            <CommandSeparator />
            <CommandGroup heading="Wilayah">
              {KABUPATEN.map((k) => (
                <CommandItem key={k.kode} value={`wilayah ${namaWilayah(k.kode)} ${k.ibukota} ${k.kode}`} onSelect={() => run(go(`/peta?wilayah=${k.kode}`))} className="rounded-xl">
                  <MapPin className="size-4 text-muted-foreground" />
                  {namaWilayah(k.kode)}
                  <span className="num ml-auto text-xs text-muted-foreground">{k.kode}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          </>
        )}
      </CommandList>
      <div className="flex items-center justify-between border-t border-border px-4 py-2.5 text-xs text-muted-foreground">
        <span className="inline-flex items-center gap-1.5"><Kbd>↑</Kbd><Kbd>↓</Kbd> pilih <Kbd><CornerDownLeft className="size-3" /></Kbd> jalankan</span>
        <span className="inline-flex items-center gap-1.5"><Kbd>Esc</Kbd> tutup</span>
      </div>
    </CommandDialog>
  );
}

export function Kbd({ children }: { children: React.ReactNode }) {
  return <kbd className="num inline-grid h-5 min-w-5 place-items-center rounded-md border border-border bg-white/70 px-1 text-[10px] text-muted-foreground shadow-ambient-xs dark:bg-white/5">{children}</kbd>;
}
