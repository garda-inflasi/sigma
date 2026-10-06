import { Suspense } from "react";
import { PrediksiPage } from "@/components/prediksi/prediksi-page";

export const metadata = { title: "Prediksi & XAI" };

export default function Page() {
  return (
    <Suspense>
      <PrediksiPage />
    </Suspense>
  );
}
