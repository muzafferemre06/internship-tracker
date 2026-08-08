# Repository working rules

- Keep product, architecture, testing, and setup documentation in sync with every code change. Documentation updates belong in the same commit as the behavior they describe.
- Split work into small, meaningful commits that each leave the repository in a coherent state.
- Use the commit subject format `<çalışılan kısım> : <bir cümlelik açıklama>`.
- Do not commit secrets, local configuration, generated build output, databases, or dependency directories.
- Prefer fixture-based scraper tests and fake providers; normal tests must not call live career sites or paid AI APIs.
- Complete and verify the current delivery phase's exit criteria before starting the next phase.
- Test-first: for any new behavior, write the failing test(s) (fixtures, fake-provider cases, table-driven cases per the plan agreed with the user) before writing the implementation. Confirm the tests fail for the expected reason, then implement until they pass. Do not write implementation code first and backfill tests after.
- Before implementing a new phase or non-trivial feature, explain the concrete plan (what will be built, what will be tested, what's out of scope) and get explicit user acknowledgement before writing code. One phase at a time.
