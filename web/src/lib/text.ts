export function getInitials(name: string): string {
  if (!name) return "?";
  const parts = name.split(" ");
  const first = parts[0];
  const last = parts[parts.length - 1];
  if (parts.length >= 2 && first && last) {
    return (first.charAt(0) + last.charAt(0)).toUpperCase();
  }
  return name.slice(0, 2).toUpperCase();
}
