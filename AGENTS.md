# Repository working rules

- Keep product, architecture, testing, and setup documentation in sync with every code change. Documentation updates belong in the same commit as the behavior they describe.
- Split work into small, meaningful commits that each leave the repository in a coherent state.
- Use the commit subject format `<çalışılan kısım> : <bir cümlelik açıklama>`.
- Push every completed commit to the configured upstream immediately after creating it. If the push fails, stop treating the delivery as complete and tell the user the exact blocker.
- Treat production deployment as a separate approval gate from commit and push. After the requested changes are committed, pushed, and the complete quality suite has passed, ask the user separately at the end of the session whether the verified revision should be deployed to production. Never infer deployment approval from permission to commit or push. Deploy only after explicit user approval, take the required pre-deploy snapshot, deploy the exact verified revision, and report production health, smoke-test, and persistent database identity results.
- Do not commit secrets, local configuration, generated build output, databases, or dependency directories.
- Prefer fixture-based scraper tests and fake providers; normal tests must not call live career sites or paid AI APIs.
- Complete and verify the current delivery phase's exit criteria before starting the next phase.
- Run the complete repository quality suite before declaring work complete, including backend tests, race tests, vet, builds, frontend tests/typecheck/build/audit, vulnerability checks, deployment contract tests, and real Docker image/Compose checks. If Docker or any required tool is unavailable, explicitly tell the user what is missing so the environment can be fixed; do not silently substitute a narrower check.
- Test-first: for any new behavior, write the failing test(s) (fixtures, fake-provider cases, table-driven cases per the plan agreed with the user) before writing the implementation. Confirm the tests fail for the expected reason, then implement until they pass. Do not write implementation code first and backfill tests after.
- Before implementing a new phase or non-trivial feature, explain the concrete plan (what will be built, what will be tested, what's out of scope) and get explicit user acknowledgement before writing code. One phase at a time.
