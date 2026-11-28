type Passage = { start_line: number; end_line: number; text: string };
export type Evidence = { source: string; sha256: string; notice: string; matches: Passage[]; more_matches: boolean; query_terms?: unknown; excluded_terms?: unknown; query_matches?: unknown; excluded_matches?: unknown };
export function parseGuideEvidence(raw: string): Evidence | null {
  try {
    const value = JSON.parse(raw);
    if (!value || typeof value.source !== "string" || !value.source.trim() ||
      typeof value.sha256 !== "string" || !/^[a-f0-9]{64}$/.test(value.sha256) ||
      typeof value.notice !== "string" || typeof value.more_matches !== "boolean" ||
      !Array.isArray(value.matches) || value.matches.length > 3 ||
      value.matches.some((p: Passage) => !p || !Number.isSafeInteger(p.start_line) ||
        !Number.isSafeInteger(p.end_line) || p.start_line < 1 || p.end_line < p.start_line ||
        typeof p.text !== "string" || !p.text.trim())) return null;
    return value;
  } catch { return null; }
}
export function guideFilters(evidence: Evidence | null) {
  const query = evidence?.query_terms;
  const excluded = evidence?.excluded_terms;
  const valid = (terms: unknown): terms is string[] =>
    Array.isArray(terms) && terms.length <= 6 &&
    terms.every(term => typeof term === "string" && term.trim().length > 0);
  if (!valid(query) || query.length === 0 || !valid(excluded)) return null;
  return { query, excluded };
}
export function guideCoverage(e: Evidence | null) {

  if (!e || typeof e.query_matches !== "number" || typeof e.excluded_matches !== "number" ||
      !Number.isSafeInteger(e.query_matches) || !Number.isSafeInteger(e.excluded_matches) ||
      e.excluded_matches < 0 || e.query_matches < e.excluded_matches) return null;
  const remaining = e.query_matches - e.excluded_matches;
  const omitted = remaining - e.matches.length;
  if (omitted < 0 || e.more_matches !== (omitted > 0)) return null;
  return { remaining, omitted };
}
