"use client";

import { useEffect, useState } from "react";

/** Nilai yang baru berubah setelah `delay` ms tanpa perubahan — untuk pencarian ketik-langsung. */
export function useDebouncedValue<T>(value: T, delay = 300): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(id);
  }, [value, delay]);
  return debounced;
}
