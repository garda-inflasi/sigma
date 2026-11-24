import { Suspense } from "react";
import { PriceInputForm } from "@/components/lapangan/price-input-form";
import { Shimmer } from "@/components/garda/shimmer";

export const metadata = { title: "Input harga" };

export default function InputHargaPage() {
  return (
    <>
      <h1 className="mb-3 text-display-sm font-semibold">
        Input harga komoditas
      </h1>
      <Suspense fallback={<Shimmer className="h-[600px] w-full rounded-xl" />}>
        <PriceInputForm />
      </Suspense>
    </>
  );
}
