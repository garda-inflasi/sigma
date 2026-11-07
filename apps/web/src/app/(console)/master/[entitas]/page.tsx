import { notFound } from "next/navigation";
import { MasterPage } from "@/components/master/master-page";
import { ENTITAS, adalahEntitas } from "@/lib/data/master";

export const metadata = { title: "Master data" };

export function generateStaticParams() {
  return Object.keys(ENTITAS).map((entitas) => ({ entitas }));
}

export default async function Page({
  params,
}: {
  params: Promise<{ entitas: string }>;
}) {
  const { entitas } = await params;
  if (!adalahEntitas(entitas)) notFound();
  return <MasterPage key={entitas} entitas={entitas} />;
}
