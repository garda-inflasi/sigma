"use client";

import { Eye, EyeOff, Lock } from "lucide-react";
import { useState } from "react";
import { idGalatField } from "@/components/form/form-bff";
import { FormField } from "@/components/form/form-field";
import { Input } from "@/components/ui/input";

/** Isian kata sandi berlabel dengan ikon kunci dan toggle tampilkan/sembunyikan. */
export function KolomSandi({
  id,
  label,
  value,
  onChange,
  disabled,
  autoComplete,
  galat,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (v: string) => void;
  disabled: boolean;
  autoComplete: string;
  /** Pesan galat field ini (skema klien atau 400/422 server). */
  galat?: string;
}) {
  const [terlihat, setTerlihat] = useState(false);

  return (
    <FormField id={id} label={label} galat={galat}>
      <div className="relative">
        <Lock className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          id={id}
          type={terlihat ? "text" : "password"}
          autoComplete={autoComplete}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          disabled={disabled}
          aria-invalid={galat ? true : undefined}
          aria-describedby={galat ? idGalatField(id) : undefined}
          className="h-11 pr-11 pl-10"
          required
        />
        <button
          type="button"
          onClick={() => setTerlihat((v) => !v)}
          disabled={disabled}
          aria-label={terlihat ? "Sembunyikan kata sandi" : "Tampilkan kata sandi"}
          aria-pressed={terlihat}
          className="absolute top-1/2 right-1.5 grid size-8 -translate-y-1/2 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none disabled:pointer-events-none disabled:opacity-40"
        >
          {terlihat ? (
            <EyeOff className="size-4" aria-hidden />
          ) : (
            <Eye className="size-4" aria-hidden />
          )}
        </button>
      </div>
    </FormField>
  );
}
