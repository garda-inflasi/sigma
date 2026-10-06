import { Suspense } from "react";
import { PenggunaPage } from "@/components/pengguna/pengguna-page";

export const metadata = { title: "Pengguna & peran" };

export default function Page() {
  return (
    <Suspense>
      <PenggunaPage />
    </Suspense>
  );
}
