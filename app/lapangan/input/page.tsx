import { Suspense } from "react";
import { PriceInputForm } from "@/components/lapangan/price-input-form";
import { Shimmer } from "@/components/garda/shimmer";

export const metadata = { title: "Input harga" };

export default function InputHargaPage() {
  return (
    <>
      <h1 className="sr-only">Input harga komoditas</h1>
      <Suspense fallback={<Shimmer className="h-[600px] w-full rounded-xl" />}>
        <PriceInputForm />
      </Suspense>
    </>
  );
}
