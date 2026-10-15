import { Suspense } from "react";
import { ColdStoragePage } from "@/components/cold-storage/cs-page";

export const metadata = { title: "Cold storage & food loss" };

export default function Page() {
  return (
    <Suspense>
      <ColdStoragePage />
    </Suspense>
  );
}
