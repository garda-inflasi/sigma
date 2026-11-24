import { Suspense } from "react";
import { CommandCenterView } from "@/components/command-center/command-center-view";

export const metadata = {
  title: "Command Center Real-Time",
  description:
    "Pusat kendali stabilitas harga pangan Aceh, peta sebaran live, dan aliran peringatan dini.",
};

export default function CommandCenterPage() {
  return (
    <Suspense>
      <CommandCenterView />
    </Suspense>
  );
}
