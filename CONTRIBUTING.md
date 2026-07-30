# Contributing

Thanks for your interest in `symroom`!

## Branch strategy

- Create feature/fix branches from `main`
- Use a descriptive slug: `fix/timeout-bug`, `feat/export-endpoint`
- Open a draft PR early, mark ready when CI passes

## Commit conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` — new feature (minor bump)
- `fix:` — bug fix (patch bump)
- `docs:` — documentation only
- `test:` — test additions or changes
- `chore:` — build, CI, tooling
- `refactor:` — code change with no behavior change
- `!` or `BREAKING CHANGE` — breaking change (major bump)

## PR workflow

1. Push your branch
2. Open a draft PR with a clear description
3. Ensure all CI checks pass (lint, build, test)
4. Mark the PR as ready for review
5. Squash-merge when approved

## Test expectations

- All PRs must pass `go test ./...`
- New features should include tests
- Bug fixes should include a regression test
- Run `gofmt -s -w .` and `go vet ./...` before pushing

## License

By contributing you agree that your contributions will be licensed under Apache-2.0.
