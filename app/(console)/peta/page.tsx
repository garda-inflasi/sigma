import { Suspense } from "react";
import { PetaPage } from "@/components/spasial/peta-page";

export const metadata = { title: "Peta surplus–defisit" };

export default function Page() {
  return (
    <Suspense>
      <PetaPage />
    </Suspense>
  );
}
