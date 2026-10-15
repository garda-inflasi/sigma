import { Suspense } from "react";
import { HargaPage } from "@/components/harga/harga-page";

export const metadata = { title: "Validasi harga" };

export default function Page() {
  return (
    <Suspense>
      <HargaPage />
    </Suspense>
  );
}
