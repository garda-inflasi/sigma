"use client";

import { AnimatePresence, motion } from "motion/react";
import {
  History,
  KeyRound,
  MonitorSmartphone,
  ShieldCheck,
  UserRound,
  type LucideIcon,
} from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useRef, type KeyboardEvent } from "react";
import { HalamanPas } from "@/components/garda/halaman-pas";
import { PageHeader } from "@/components/garda/page-header";
import { MODE_MOCK } from "@/lib/api/client";
import { useProfil } from "@/lib/query/profil-hooks";
import { ease, spring } from "@/lib/motion/presets";
import {
  LABEL_TAB_PROFIL,
  TAB_BAWAAN,
  TAB_PROFIL,
  bacaTabProfil,
  hrefTabProfil,
  type TabProfil,
} from "@/lib/profil/tab";
import { cn } from "@/lib/utils";
import { TabAktivitas } from "./tab-aktivitas";
import { TabGantiSandi } from "./tab-ganti-sandi";
import { TabMfa } from "./tab-mfa";
import { TabProfil as TabDataDiri } from "./tab-profil";
import { TabSesi } from "./tab-sesi";

const IKON_TAB: Record<TabProfil, LucideIcon> = {
  profil: UserRound,
  "ganti-password": KeyRound,
  mfa: ShieldCheck,
  keamanan: MonitorSmartphone,
  aktivitas: History,
};

// Mode mock tidak punya BFF auth: hanya data diri (dari sesi) yang bisa ditampilkan.
const TAB_TERSEDIA: readonly TabProfil[] = MODE_MOCK
  ? [TAB_BAWAAN]
  : TAB_PROFIL;

const idTab = (tab: TabProfil) => `tab-profil-${tab}`;
const ID_PANEL = "panel-profil";

const TRANSISI_PANEL = {
  hidden: { opacity: 0, y: 6 },
  show: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.22, ease: ease.garda },
  },
  exit: {
    opacity: 0,
    y: -4,
    transition: { duration: 0.14, ease: ease.inOut },
  },
};

function IsiTab({ tab }: { tab: TabProfil }) {
  switch (tab) {
    case "ganti-password":
      return <TabGantiSandi />;
    case "mfa":
      return <TabMfa />;
    case "keamanan":
      return <TabSesi />;
    case "aktivitas":
      return <TabAktivitas />;
    default:
      return <TabDataDiri />;
  }
}

/**
 * Kerangka /profil: tab sebaris dengan judul (bergulir mendatar di mobile) dan panel isi yang mengikuti ruang.
 * Tab aktif selalu diturunkan dari `?tab=`, jadi tautan menu akun di topbar dan tombol kembali peramban ikut
 * menggantinya; klik tab memakai replaceState agar riwayat tidak menumpuk.
 */
export function ProfilPage() {
  const params = useSearchParams();
  const diminta = bacaTabProfil(params.get("tab"));
  const aktif = TAB_TERSEDIA.includes(diminta) ? diminta : TAB_BAWAAN;
  const tombol = useRef<Partial<Record<TabProfil, HTMLButtonElement | null>>>(
    {},
  );
  const profilQuery = useProfil();
  const mfaAktif = profilQuery.data?.mfa_aktif;

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
    <HalamanPas>
      <PageHeader
        title="Profil saya"
        description="Kelola data pribadi, kata sandi, verifikasi dua langkah, perangkat, dan jejak aktivitas akun."
        tabs={
          <nav aria-label="Menu profil" className="min-w-0">
            <div
              role="tablist"
              aria-label="Bagian profil"
              onKeyDown={onKeyDown}
              className="no-scrollbar flex gap-1 overflow-x-auto border-b border-border pb-px"
            >
              {TAB_TERSEDIA.map((tab) => {
                const Ikon = IKON_TAB[tab];
                const dipilih = tab === aktif;
                const titikMfa =
                  tab === "mfa" && mfaAktif !== undefined ? mfaAktif : null;
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
                      "relative flex min-h-11 shrink-0 items-center gap-2 rounded-t-lg px-3.5 text-[13px] font-medium whitespace-nowrap transition-colors focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none lg:min-h-9.5",
                      dipilih
                        ? "text-foreground"
                        : "text-muted-foreground hover:bg-secondary/60 hover:text-foreground",
                    )}
                  >
                    <Ikon
                      className={cn(
                        "size-3.5 shrink-0 transition-colors",
                        dipilih ? "text-data-600 dark:text-data-400" : "",
                      )}
                      aria-hidden
                    />
                    {LABEL_TAB_PROFIL[tab]}
                    {titikMfa !== null && (
                      <span
                        aria-hidden
                        className={cn(
                          "size-1.5 rounded-full",
                          titikMfa ? "bg-normal" : "bg-waspada",
                        )}
                      />
                    )}
                    {dipilih && (
                      <motion.span
                        layoutId="profil-tab-aktif"
                        transition={spring.layout}
                        className="absolute inset-x-1.5 bottom-0 h-0.5 rounded-full bg-navy-800 dark:bg-data-400"
                      />
                    )}
                  </button>
                );
              })}
            </div>
          </nav>
        }
      />

      <div
        role="tabpanel"
        id={ID_PANEL}
        aria-labelledby={idTab(aktif)}
        tabIndex={0}
        className="flex min-w-0 flex-col focus-visible:outline-none lg:min-h-0 lg:flex-1"
      >
        <AnimatePresence mode="wait" initial={false}>
          <motion.div
            key={aktif}
            variants={TRANSISI_PANEL}
            initial="hidden"
            animate="show"
            exit="exit"
            className="flex min-w-0 flex-col lg:min-h-0 lg:flex-1"
          >
            <IsiTab tab={aktif} />
          </motion.div>
        </AnimatePresence>
      </div>
    </HalamanPas>
  );
}
