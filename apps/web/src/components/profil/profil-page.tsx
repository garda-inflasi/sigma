"use client";

import { AnimatePresence, motion } from "motion/react";
import {
  KeyRound,
  MonitorSmartphone,
  ShieldCheck,
  UserRound,
  type LucideIcon,
} from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useRef, type KeyboardEvent } from "react";
import { PageHeader } from "@/components/garda/page-header";
import { MODE_MOCK } from "@/lib/api/client";
import { fadeIn, spring } from "@/lib/motion/presets";
import {
  LABEL_TAB_PROFIL,
  TAB_BAWAAN,
  TAB_PROFIL,
  bacaTabProfil,
  hrefTabProfil,
  type TabProfil,
} from "@/lib/profil/tab";
import { cn } from "@/lib/utils";
import { TabGantiSandi } from "./tab-ganti-sandi";
import { TabMfa } from "./tab-mfa";
import { TabProfil as TabDataDiri } from "./tab-profil";
import { TabSesi } from "./tab-sesi";

const IKON_TAB: Record<TabProfil, LucideIcon> = {
  profil: UserRound,
  "ganti-password": KeyRound,
  mfa: ShieldCheck,
  keamanan: MonitorSmartphone,
};

// Mode mock tidak punya BFF auth: hanya data diri (dari sesi) yang bisa ditampilkan.
const TAB_TERSEDIA: readonly TabProfil[] = MODE_MOCK
  ? [TAB_BAWAAN]
  : TAB_PROFIL;

const idTab = (tab: TabProfil) => `tab-profil-${tab}`;
const ID_PANEL = "panel-profil";

function IsiTab({ tab }: { tab: TabProfil }) {
  switch (tab) {
    case "ganti-password":
      return <TabGantiSandi />;
    case "mfa":
      return <TabMfa />;
    case "keamanan":
      return <TabSesi />;
    default:
      return <TabDataDiri />;
  }
}

/**
 * Kerangka /profil: navigasi tab (vertikal di desktop, pil bergulir di mobile) dan panel isi. Tab aktif
 * selalu diturunkan dari `?tab=`, jadi tautan menu akun di topbar dan tombol kembali peramban ikut
 * menggantinya; klik tab memakai replaceState agar riwayat tidak menumpuk.
 */
export function ProfilPage() {
  const params = useSearchParams();
  const diminta = bacaTabProfil(params.get("tab"));
  const aktif = TAB_TERSEDIA.includes(diminta) ? diminta : TAB_BAWAAN;
  const tombol = useRef<Partial<Record<TabProfil, HTMLButtonElement | null>>>(
    {},
  );

  const pilih = (tab: TabProfil) => {
    if (tab !== aktif)
      window.history.replaceState(null, "", hrefTabProfil(tab));
  };

  // Pola tablist WAI-ARIA: panah pindah tab dan fokus, Home/End ke ujung.
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const i = TAB_TERSEDIA.indexOf(aktif);
    const n = TAB_TERSEDIA.length;
    const tujuan: Record<string, number> = {
      ArrowDown: (i + 1) % n,
      ArrowRight: (i + 1) % n,
      ArrowUp: (i - 1 + n) % n,
      ArrowLeft: (i - 1 + n) % n,
      Home: 0,
      End: n - 1,
    };
    if (!(e.key in tujuan)) return;
    e.preventDefault();
    const tab = TAB_TERSEDIA[tujuan[e.key]];
    pilih(tab);
    tombol.current[tab]?.focus();
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Profil saya"
        description="Kelola data diri, kata sandi, verifikasi dua langkah, dan perangkat yang sedang masuk ke akun Anda."
      />

      <div className="grid gap-5 lg:grid-cols-[248px_minmax(0,1fr)] lg:items-start">
        {/* min-w-0: item grid tidak boleh melebar mengikuti pil tab, supaya pil bergulir di dalam nav (mobile). */}
        <nav aria-label="Menu profil" className="min-w-0 lg:sticky lg:top-24">
          <div
            role="tablist"
            aria-label="Bagian profil"
            onKeyDown={onKeyDown}
            className="glass no-scrollbar -mx-1 flex gap-1 overflow-x-auto rounded-xl p-1.5 lg:mx-0 lg:flex-col lg:overflow-visible"
          >
            {TAB_TERSEDIA.map((tab) => {
              const Ikon = IKON_TAB[tab];
              const dipilih = tab === aktif;
              return (
                <button
                  key={tab}
                  ref={(el) => {
                    tombol.current[tab] = el;
                  }}
                  type="button"
                  role="tab"
                  id={idTab(tab)}
                  aria-selected={dipilih}
                  aria-controls={ID_PANEL}
                  tabIndex={dipilih ? 0 : -1}
                  onClick={() => pilih(tab)}
                  className={cn(
                    "relative flex min-h-11 shrink-0 items-center gap-2.5 rounded-lg px-3.5 text-sm font-medium whitespace-nowrap transition-colors focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none",
                    dipilih
                      ? "text-navy-900 dark:text-white"
                      : "text-muted-foreground hover:bg-secondary/70 hover:text-navy-800 dark:hover:text-white",
                  )}
                >
                  {dipilih && (
                    <motion.span
                      layoutId="profil-tab-aktif"
                      transition={spring.layout}
                      className="absolute inset-0 rounded-lg bg-white shadow-ambient-sm dark:bg-white/10"
                    />
                  )}
                  <Ikon
                    className={cn(
                      "relative size-4 shrink-0",
                      dipilih && "text-data-600 dark:text-data-400",
                    )}
                  />
                  <span className="relative">{LABEL_TAB_PROFIL[tab]}</span>
                </button>
              );
            })}
          </div>
        </nav>

        <div
          role="tabpanel"
          id={ID_PANEL}
          aria-labelledby={idTab(aktif)}
          tabIndex={0}
          className="min-h-[420px] min-w-0 focus-visible:outline-none"
        >
          <AnimatePresence mode="wait" initial={false}>
            <motion.div
              key={aktif}
              variants={fadeIn}
              initial="hidden"
              animate="show"
              exit="exit"
            >
              <IsiTab tab={aktif} />
            </motion.div>
          </AnimatePresence>
        </div>
      </div>
    </div>
  );
}
