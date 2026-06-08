# Ebooks Work Handoff

Date: 2026-06-07

## Workspace

- Worktree: `/Users/jimcole/.config/superpowers/worktrees/silo-server/ebooks-on-audiobooks`
- Branch: `work/ebooks-on-audiobooks`
- Upstream: `origin/work/ebooks-on-audiobooks`
- Latest commit: `bfabec3 feat(ebooks): add frontend catalog scope controls`
- Worktree was clean before this handoff note was added.

## Current Status

Tasks 1 through 7 are implemented and committed:

1. Ebook details schema/repository.
2. Local ebook parser foundation.
3. Ebook library routing to the ebook scanner.
4. Ebook catalog/details/ISBN/author upserts.
5. Unchanged-file skipping, missing detection, cover cache.
6. Backend catalog ebook scope/facets.
7. Frontend catalog ebook scope controls.

Task 8 is still pending: final review/branch prep/PR decision.

## Important Domain Correction

Ebooks do not have narrators. Audiobooks do.

Confirmed/implemented:

- Ebook scanner writes author people only in `internal/scanner/ebook_scan.go`.
- Audiobook scanner writes author plus narrator people in `internal/scanner/audiobook_scan.go`.
- Backend catalog ebook scope lists/searches authors and ebook series, but blocks narrator facet search.
- Frontend ebook scope/library shows author and series book facets, but does not render or emit narrator rules.
- Audiobook narrator behavior remains available.

## Recent Commits

- `bfabec3 feat(ebooks): add frontend catalog scope controls`
- `fb47141 fix(ebooks): normalize catalog ebook scope`
- `f06c183 feat(ebooks): expose ebooks in catalog facets`
- `4dd53b6 fix(ebooks): repair unchanged scan prerequisites`
- `58d9fa8 feat(ebooks): skip unchanged files and cache covers`
- `7471d73 fix(ebooks): refresh scan-owned ebook metadata`
- `f6a27c2 feat(ebooks): upsert ebook catalog rows`
- `6267b49 fix(ebooks): use logical walker for ebook scans`
- `34eb022 feat(ebooks): route ebook libraries to scanner`

## Verification Already Run

Backend:

```sh
go test ./internal/scanner ./internal/catalog ./migrations -count=1
```

Passed.

Frontend:

```sh
pnpm --filter silo-web test src/components/collections/CollectionGuidedRulesEditor.test.tsx src/lib/querySortOptions.test.ts src/pages/libraryPageSearchParams.test.ts src/pages/catalogSearchParams.test.ts
pnpm --filter silo-web build
```

Passed. Focused frontend run was 4 files / 46 tests. The build still emits the existing Vite large chunk warning and pnpm workspace warning.

## Review State

Task 6 backend review passed after `fb47141`.

Task 7 frontend implementation was committed and verified locally. It also includes regression guards so stale hidden narrator rules are dropped when editing ebook library filters, and ebook libraries do not show narrator even if stale audiobook scope is present. Two review agents were started for Task 7 spec/code-quality review, but the user asked to stop/disconnect before results returned. Those agents were closed/interrupted. When resuming, rerun or redo Task 7 review before calling the branch ready.

## Next Steps

1. Review `a367474` for Task 7 frontend behavior.
2. Rerun final verification if anything changes.
3. Decide whether to push/update the remote branch and create the PR/MR.
4. Keep diagnostics/separate follow-up work out of this PR unless explicitly requested.
