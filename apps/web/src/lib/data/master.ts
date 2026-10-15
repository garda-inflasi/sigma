export const ENTITAS = { komoditas: "Komoditas", wilayah: "Wilayah (kode BPS)", pasar: "Pasar" } as const;
export type Entitas = keyof typeof ENTITAS;
