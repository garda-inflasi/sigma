import type { ComponentProps, ReactNode } from "react";
import { PesanField } from "@/components/form/form-bff";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

/**
 * Satu isian form: label, kontrol selebar kolom, teks bantuan, dan pesan galat. `id` harus sama dengan id
 * kontrolnya; pesan galat memakai `idGalatField(id)` sehingga `aria-describedby` kontrol tetap berlaku.
 */
export function FormField({
  id,
  label,
  wajib,
  opsional,
  bantuan,
  galat,
  className,
  children,
}: {
  id: string;
  label: ReactNode;
  /** Menampilkan tanda bintang (disembunyikan dari pembaca layar; pakai `aria-required` di kontrol). */
  wajib?: boolean;
  /** Menambahkan "(opsional)" setelah label. */
  opsional?: boolean;
  bantuan?: ReactNode;
  galat?: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={cn("flex min-w-0 flex-col gap-1.5", className)}>
      <Label htmlFor={id} className="gap-1 leading-snug">
        <span>
          {label}
          {opsional && (
            <span className="font-normal text-muted-foreground">
              {" "}
              (opsional)
            </span>
          )}
        </span>
        {wajib && (
          <span aria-hidden className="text-destructive">
            *
          </span>
        )}
      </Label>
      {children}
      {bantuan && <p className="text-xs text-muted-foreground">{bantuan}</p>}
      <PesanField field={id} pesan={galat} />
    </div>
  );
}

/** Banner galat tingkat form. Tidak merender apa pun tanpa isi. */
export function FormAlert({
  className,
  children,
  ...props
}: ComponentProps<"div">) {
  if (!children) return null;
  return (
    <div
      role="alert"
      className={cn(
        "min-w-0 rounded-lg border border-defisit/25 bg-defisit-soft px-3 py-2 text-[13px] break-words text-defisit-ink dark:bg-defisit/10 dark:text-defisit",
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}
