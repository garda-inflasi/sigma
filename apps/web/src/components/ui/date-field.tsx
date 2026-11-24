"use client";

import { CalendarDays } from "lucide-react";
import {
  useEffect,
  useId,
  useRef,
  useState,
  type ChangeEvent,
  type ComponentProps,
  type FocusEvent,
  type KeyboardEvent,
} from "react";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverAnchor,
  PopoverContent,
} from "@/components/ui/popover";
import {
  bacaKetikanTanggal,
  bacaKetikanTanggalWaktu,
  bagianWaktu,
  dateDariIso,
  galatRentangTanggal,
  isoDariDate,
  rapikanKetikanTanggal,
  tampilTanggal,
  tampilTanggalWaktu,
} from "@/lib/tanggal-input";
import { cn } from "@/lib/utils";

const TAHUN_LALU = 12;
const TAHUN_DEPAN = 10;
const dua = (n: number) => String(n).padStart(2, "0");
const jamSekarang = (d = new Date()) =>
  `${dua(d.getHours())}:${dua(d.getMinutes())}:${dua(d.getSeconds())}`;

type PropsField = Omit<
  ComponentProps<"input">,
  "type" | "value" | "defaultValue" | "onChange" | "min" | "max" | "step"
> & {
  value: string;
  onValueChange: (nilai: string) => void;
  /** Kelas pembungkus: atur lebar field di sini. */
  className?: string;
  /** Kelas elemen input: ukuran dan tipografi. */
  kelasInput?: string;
};

/** Field tanggal dengan kalender. Nilai tetap `YYYY-MM-DD`, atau `""` bila kosong. */
export function DateField({
  min,
  max,
  ...props
}: PropsField & { min?: string; max?: string }) {
  return <KolomTanggal denganWaktu={false} min={min} max={max} {...props} />;
}

/** Field tanggal dan waktu setempat dengan kalender. Nilai tetap `YYYY-MM-DDTHH:mm:ss`. */
export function DateTimeField(props: PropsField) {
  return <KolomTanggal denganWaktu {...props} />;
}

function KolomTanggal({
  denganWaktu,
  value,
  onValueChange,
  min,
  max,
  className,
  kelasInput,
  disabled,
  placeholder,
  onBlur,
  "aria-invalid": ariaInvalid,
  "aria-describedby": ariaDescribedBy,
  ...props
}: PropsField & { denganWaktu: boolean; min?: string; max?: string }) {
  const idGalat = useId();
  const wadah = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLInputElement>(null);
  const tombol = useRef<HTMLButtonElement>(null);
  const sentuh = useRef(false);
  const tujuanFokus = useRef<HTMLElement | null>(null);

  const [buka, setBuka] = useState(false);
  const [fokusHari, setFokusHari] = useState(false);
  const [draf, setDraf] = useState<string | null>(null);
  const [tampilGalat, setTampilGalat] = useState(false);
  const [bulan, setBulan] = useState<Date | undefined>(() =>
    dateDariIso(value),
  );
  const [nilaiLalu, setNilaiLalu] = useState(value);

  // Nilai berubah dari luar (pilih kalender, reset filter): ketikan lama tidak berlaku lagi.
  if (value !== nilaiLalu) {
    setNilaiLalu(value);
    setDraf(null);
    setTampilGalat(false);
    const d = dateDariIso(value);
    if (d) setBulan(d);
  }

  const baca = denganWaktu ? bacaKetikanTanggalWaktu : bacaKetikanTanggal;
  const tampil =
    draf ?? (denganWaktu ? tampilTanggalWaktu(value) : tampilTanggal(value));
  const terbaca = draf?.trim() ? baca(draf) : null;
  const pesanGalat = draf?.trim()
    ? terbaca
      ? galatRentangTanggal(terbaca, { min, max })
      : denganWaktu
        ? "Tulis sebagai hh/bb/tttt jj:mm, contoh 24/11/2026 08:30."
        : "Tulis sebagai hh/bb/tttt, contoh 24/11/2026."
    : null;
  const galatTerlihat = tampilGalat && !!pesanGalat;

  // Ketikan yang salah tidak boleh terkirim sebagai nilai lama atau kosong.
  useEffect(() => {
    const form = input.current?.form;
    if (!form || !pesanGalat) return;
    const tahan = (e: Event) => {
      e.preventDefault();
      e.stopImmediatePropagation();
      setTampilGalat(true);
      input.current?.focus();
    };
    form.addEventListener("submit", tahan, true);
    return () => form.removeEventListener("submit", tahan, true);
  }, [pesanGalat]);

  const tanggalMin = dateDariIso(min);
  const tanggalMax = dateDariIso(max);
  const terpilih = dateDariIso(value);
  const sekarang = new Date();
  const hariIni = isoDariDate(sekarang);
  const hariIniBoleh = !galatRentangTanggal(hariIni, { min, max });
  const tahunAwal = Math.min(
    tanggalMin?.getFullYear() ?? sekarang.getFullYear() - TAHUN_LALU,
    terpilih?.getFullYear() ?? sekarang.getFullYear(),
  );
  const tahunAkhir = Math.max(
    tanggalMax?.getFullYear() ?? sekarang.getFullYear() + TAHUN_DEPAN,
    terpilih?.getFullYear() ?? sekarang.getFullYear(),
  );

  function bukaKalender(keHari: boolean, asal: HTMLElement | null) {
    if (disabled) return;
    tujuanFokus.current = asal;
    setFokusHari(keHari);
    setBulan(
      terpilih ??
        (tanggalMax && tanggalMax < sekarang
          ? tanggalMax
          : tanggalMin && tanggalMin > sekarang
            ? tanggalMin
            : sekarang),
    );
    setBuka(true);
  }

  function ketik(e: ChangeEvent<HTMLInputElement>) {
    const teks = rapikanKetikanTanggal(e.target.value, tampil);
    setDraf(teks);
    setTampilGalat(false);
    if (!teks.trim()) {
      if (value) onValueChange("");
      return;
    }
    const n = baca(teks);
    if (!n || galatRentangTanggal(n, { min, max })) return;
    const d = dateDariIso(n);
    if (d) setBulan(d);
    if (n !== value) onValueChange(n);
  }

  function lepas(e: FocusEvent<HTMLInputElement>) {
    onBlur?.(e);
    if (draf === null) return;
    if (!pesanGalat) setDraf(null);
    // Di dalam form ketikan dipertahankan agar bisa dibetulkan; di filter kembali ke nilai yang sedang berlaku.
    else if (e.currentTarget.form) setTampilGalat(true);
    else setDraf(null);
  }

  function tekan(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowDown" && !buka) {
      e.preventDefault();
      bukaKalender(true, input.current);
    } else if (e.key === "Enter" && buka) {
      e.preventDefault();
      setBuka(false);
    }
  }

  /** Nilai filter di URL baru berubah setelah navigasi, jadi pilihan ditampilkan lebih dulu sebagai draf. */
  function terapkan(nilai: string) {
    setDraf(denganWaktu ? tampilTanggalWaktu(nilai) : tampilTanggal(nilai));
    setTampilGalat(false);
    onValueChange(nilai);
  }

  function pilihHari(d: Date | undefined) {
    if (!d) return;
    const iso = isoDariDate(d);
    if (denganWaktu) {
      terapkan(`${iso}T${bagianWaktu(value) || jamSekarang()}`);
      return;
    }
    terapkan(iso);
    setBuka(false);
  }

  function ubahJam(e: ChangeEvent<HTMLInputElement>) {
    if (!e.target.value) return;
    const jam =
      e.target.value.length === 5 ? `${e.target.value}:00` : e.target.value;
    terapkan(`${terpilih ? isoDariDate(terpilih) : hariIni}T${jam}`);
  }

  function pakaiSekarang() {
    const d = new Date();
    setBulan(d);
    terapkan(
      denganWaktu ? `${isoDariDate(d)}T${jamSekarang(d)}` : isoDariDate(d),
    );
    if (!denganWaktu) setBuka(false);
  }

  return (
    <div className={cn("w-full min-w-0", className)}>
      <Popover open={buka} onOpenChange={setBuka}>
        <PopoverAnchor asChild>
          <div ref={wadah} className="relative">
            <Input
              {...props}
              ref={input}
              type="text"
              inputMode={denganWaktu ? "text" : "numeric"}
              autoComplete="off"
              disabled={disabled}
              value={tampil}
              placeholder={
                placeholder ??
                (denganWaktu ? "hh/bb/tttt jj:mm:dd" : "hh/bb/tttt")
              }
              aria-haspopup="dialog"
              aria-expanded={buka}
              aria-invalid={ariaInvalid || galatTerlihat || undefined}
              aria-describedby={
                cn(ariaDescribedBy, galatTerlihat && idGalat) || undefined
              }
              onChange={ketik}
              onBlur={lepas}
              onKeyDown={tekan}
              onPointerDown={(e) => {
                sentuh.current = e.pointerType === "touch";
              }}
              onMouseDown={(e) => {
                // Di layar sentuh kalender dipakai langsung, tanpa memunculkan papan ketik.
                if (sentuh.current) e.preventDefault();
              }}
              onClick={() => {
                if (!buka)
                  bukaKalender(
                    sentuh.current,
                    sentuh.current ? tombol.current : input.current,
                  );
              }}
              className={cn("num pr-9", kelasInput)}
            />
            <button
              ref={tombol}
              type="button"
              disabled={disabled}
              aria-label="Buka kalender"
              aria-haspopup="dialog"
              aria-expanded={buka}
              onClick={() =>
                buka ? setBuka(false) : bukaKalender(true, tombol.current)
              }
              className="absolute inset-y-0 right-0 grid w-9 place-items-center rounded-r-lg text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/40 disabled:pointer-events-none disabled:opacity-50"
            >
              <CalendarDays aria-hidden className="size-4" />
            </button>
          </div>
        </PopoverAnchor>
        <PopoverContent
          align="start"
          aria-label={denganWaktu ? "Pilih tanggal dan waktu" : "Pilih tanggal"}
          className="z-[70] w-auto max-w-[calc(100vw-1rem)] p-2.5"
          collisionPadding={8}
          onOpenAutoFocus={(e) => e.preventDefault()}
          onCloseAutoFocus={(e) => {
            e.preventDefault();
            tujuanFokus.current?.focus({ preventScroll: true });
            tujuanFokus.current = null;
          }}
          onInteractOutside={(e) => {
            const t = e.detail.originalEvent.target;
            if (t instanceof Node && wadah.current?.contains(t))
              e.preventDefault();
            else tujuanFokus.current = null;
          }}
        >
          <Calendar
            mode="single"
            required
            autoFocus={fokusHari}
            selected={terpilih}
            onSelect={pilihHari}
            month={bulan}
            onMonthChange={setBulan}
            startMonth={new Date(tahunAwal, 0)}
            endMonth={new Date(tahunAkhir, 11)}
            disabled={[
              ...(tanggalMin ? [{ before: tanggalMin }] : []),
              ...(tanggalMax ? [{ after: tanggalMax }] : []),
            ]}
          />
          {denganWaktu && (
            <label className="mt-2 flex items-center gap-2 border-t border-border pt-2 text-[13px] font-medium">
              Jam
              <Input
                type="time"
                step={1}
                value={bagianWaktu(value)}
                onChange={ubahJam}
                className="num h-8 flex-1 md:text-[13px] dark:scheme-dark"
              />
            </label>
          )}
          <div className="mt-2 flex items-center justify-between gap-2 border-t border-border pt-2">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              disabled={!hariIniBoleh}
              onClick={pakaiSekarang}
            >
              {denganWaktu ? "Sekarang" : "Hari ini"}
            </Button>
            {denganWaktu ? (
              <Button
                type="button"
                variant="secondary"
                size="sm"
                onClick={() => {
                  tujuanFokus.current ??= input.current;
                  setBuka(false);
                }}
              >
                Selesai
              </Button>
            ) : (
              value && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    terapkan("");
                    setBuka(false);
                  }}
                >
                  Hapus
                </Button>
              )
            )}
          </div>
        </PopoverContent>
      </Popover>
      {galatTerlihat && (
        <p id={idGalat} role="alert" className="mt-1 text-xs text-destructive">
          {pesanGalat}
        </p>
      )}
    </div>
  );
}
