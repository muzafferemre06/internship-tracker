# Repository working rules

- Keep product, architecture, testing, and setup documentation in sync with every code change. Documentation updates belong in the same commit as the behavior they describe.
- Split work into small, meaningful commits that each leave the repository in a coherent state.
- Use the commit subject format `<çalışılan kısım> : <bir cümlelik açıklama>`.
- Do not commit secrets, local configuration, generated build output, databases, or dependency directories.
- Prefer fixture-based scraper tests and fake providers; normal tests must not call live career sites or paid AI APIs.
- Complete and verify the current delivery phase's exit criteria before starting the next phase.
