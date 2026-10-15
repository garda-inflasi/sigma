import { del, get, set } from "idb-keyval";
import type { StateStorage } from "zustand/middleware";

/** Penyimpanan IndexedDB untuk zustand/persist — tahan offline, kapasitas jauh di atas localStorage. */
export const idbStorage: StateStorage = {
  getItem: async (name) => (typeof indexedDB === "undefined" ? null : ((await get<string>(name)) ?? null)),
  setItem: async (name, value) => {
    if (typeof indexedDB !== "undefined") await set(name, value);
  },
  removeItem: async (name) => {
    if (typeof indexedDB !== "undefined") await del(name);
  },
};
