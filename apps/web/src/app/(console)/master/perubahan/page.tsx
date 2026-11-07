import { PerubahanPage } from "@/components/master/perubahan-page";

export const metadata = { title: "Antrean perubahan master data" };

/** Rute statis: menang atas `[entitas]` di sebelahnya, jadi "perubahan" tidak pernah dianggap entitas. */
export default function Page() {
  return <PerubahanPage />;
}
