export function formatBirthDate(dateStr: string): string {
  const date = new Date(dateStr + "T00:00:00");
  return date.toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
}

export function computeAge(birthStr: string, deathStr?: string): number {
  const birth = new Date(birthStr + "T00:00:00");
  const ref = deathStr ? new Date(deathStr + "T00:00:00") : new Date();
  let age = ref.getFullYear() - birth.getFullYear();
  const monthDiff = ref.getMonth() - birth.getMonth();
  if (monthDiff < 0 || (monthDiff === 0 && ref.getDate() < birth.getDate())) {
    age--;
  }
  return age;
}
