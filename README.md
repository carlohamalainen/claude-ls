# claude-ls

List recent Claude Code sessions — timestamp, working directory, and a snippet
of the last prompt — so you can quickly recall where you left off and start a
fresh tmux session there.

```
$ claude-ls
TIME              CWD                       SNIPPET
2026-04-27 07:10  ~/w/claude-ls             initial commit and basic CLI
2026-04-26 21:57  ~/code/example-app        refactor the parser module
2026-04-26 16:13  ~/code/another-repo       add unit tests for the new endpoint
2026-04-25 07:23  ~/scratch/prototype       explore a quick experiment
2026-04-24 20:21  ~/code/sandbox            try out a new idea
```

## Install

Pre-built binaries are published to
[GitHub Releases](https://github.com/carlohamalainen/claude-ls/releases/latest).

### macOS (Apple Silicon)

```sh
curl -L -o claude-ls \
  https://github.com/carlohamalainen/claude-ls/releases/latest/download/claude-ls-darwin-arm64
chmod +x claude-ls
```

### Linux (x86_64)

```sh
curl -L -o claude-ls \
  https://github.com/carlohamalainen/claude-ls/releases/latest/download/claude-ls-linux-amd64
chmod +x claude-ls
```

### Verify the download (optional)

Each release also publishes a `SHA256SUMS` file. To verify:

```sh
curl -L -O https://github.com/carlohamalainen/claude-ls/releases/latest/download/SHA256SUMS
shasum -a 256 -c SHA256SUMS --ignore-missing
```

### From source

```sh
go install github.com/carlohamalainen/claude-ls@latest
```

## Usage

```
claude-ls               # 20 most recent sessions
claude-ls -n 50         # 50 most recent
claude-ls -n 0          # all
claude-ls -since 24h    # only sessions touched in the last 24h
claude-ls -since 7d     # last 7 days
claude-ls -cwd myproj   # filter by cwd substring
claude-ls -all          # no truncation, no row limit
claude-ls -version
```

By default it reads `~/.claude/projects/`. Override with `-root <path>`.

## Build

```sh
make build              # local binary
make release            # cross-compiles to dist/ for darwin/arm64 and linux/amd64
```

## Releasing

See [RELEASING.md](RELEASING.md).

## License

MIT — see [LICENSE](LICENSE).
