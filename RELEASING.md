# Releasing

The GitHub Actions workflow at [.github/workflows/release.yml](.github/workflows/release.yml)
builds and publishes binaries when a tag matching `v*` is pushed.

## Cut a release

```sh
git tag v0.1.0
git push origin v0.1.0
```

The workflow then:

1. Builds `claude-ls-darwin-arm64` and `claude-ls-linux-amd64` in parallel,
   embedding the tag name as `main.version` via `-ldflags`.
2. Generates a `SHA256SUMS` file across both binaries.
3. Creates a GitHub Release named after the tag, with the two binaries and
   `SHA256SUMS` attached and auto-generated release notes.

## Versioning

Tags are plain semver: `v<MAJOR>.<MINOR>.<PATCH>`. Pre-releases (e.g. `v0.2.0-rc1`)
also match the `v*` trigger and will publish a release marked as pre-release if
the tag name follows GitHub's pre-release conventions.

## Verify locally first

Before tagging, sanity-check the cross-compile:

```sh
make release
./dist/claude-ls-darwin-arm64 -version    # only on macOS arm64
./dist/claude-ls-linux-amd64 --help        # works on Linux; on macOS just check it built
```

## Note on jj

`jj git push` does not currently push tags, only bookmarks. Use `git tag` and
`git push origin <tag>` for the tag step. The colocated git repo at `.git`
makes this seamless alongside jj.
