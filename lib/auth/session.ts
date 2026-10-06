import "server-only";
import { cookies } from "next/headers";
import { resolveSession } from "./resolve";

export async function getSession() {
  const jar = await cookies();
  return resolveSession((n) => jar.get(n)?.value);
}
