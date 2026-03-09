export interface AlertRange {
  name: string;
  startLine: number;
  endLine: number;
}

/** Parse alert rules YAML to find the full line range of each `- alert:` entry. */
export function findAlertRanges(text: string): AlertRange[] {
  const results: AlertRange[] = [];
  const lines = text.split("\n");
  for (let i = 0; i < lines.length; i++) {
    const match = lines[i]?.match(/^(\s*)-\s*alert:\s*(.+?)\s*$/);
    if (!match?.[2]) continue;
    const indent = match[1]?.length ?? 0;
    // Scan forward: entry continues while lines are empty or indented deeper.
    let end = i;
    for (let j = i + 1; j < lines.length; j++) {
      const ln = lines[j] ?? "";
      if (ln.trim() === "") { end = j; continue; }
      const leadingSpaces = ln.search(/\S/);
      if (leadingSpaces <= indent) break;
      end = j;
    }
    results.push({ name: match[2], startLine: i + 1, endLine: end + 1 }); // 1-based
  }
  return results;
}
