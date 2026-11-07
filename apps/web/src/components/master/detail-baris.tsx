import type { ReactNode } from "react";

/** Satu baris label/nilai di dalam `<dl className="grid grid-cols-[8rem_1fr] …">`. */
export function DlBaris({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <>
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd>{children}</dd>
    </>
  );
}

/** Pesan galat/kosong satu bagian sheet. */
export function CatatanSheet({
  children,
  galat,
}: {
  children: ReactNode;
  galat?: boolean;
}) {
  return (
    <p
      role={galat ? "alert" : undefined}
      className={
        galat ? "text-sm text-destructive" : "text-xs text-muted-foreground"
      }
    >
      {children}
    </p>
  );
}
