import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

/**
 * tailwind-merge yang mengenal token kustom di globals.css. Tanpa ini `text-display-md` dianggap warna teks
 * (sehingga membuang `text-foreground`) dan `shadow-ambient-*` tidak menggantikan `shadow-xs`.
 */
const twMerge = extendTailwindMerge({
  extend: {
    theme: {
      text: [
        "2xs",
        "display-2xl",
        "display-xl",
        "display-lg",
        "display-md",
        "display-sm",
      ],
      radius: ["modal"],
      shadow: [
        "ambient-xs",
        "ambient-sm",
        "ambient",
        "ambient-lg",
        "lift",
        "glow-data",
        "glow-defisit",
        "glow-waspada",
      ],
    },
  },
});

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));
