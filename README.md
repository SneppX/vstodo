# VSTodo

**A new version control system, built from the ground up in Go.**

VSTodo is an independent version control system inspired by Git. Its goal is to build a powerful, flexible, and developer-friendly tool with its own implementation of version control fundamentals.

## Vision

The long-term vision is to build a complete version control ecosystem.

- Repository management
- File tracking and staging
- Commits and history
- Branching and merging
- Diff and change detection
- Object storage and packfiles
- Remote repositories
- Push, pull, fetch, and clone
- GitHub interoperability
- A dedicated web platform

## Built With

- Go
- Git-compatible object formats and smart-HTTP protocols for interoperability

## Remote transport

VSTodo currently supports HTTP smart-transport reference discovery, fetch, clone,
fast-forward-only branch push, and fetch-plus-merge pull. For GitHub authentication, set
`VSTODO_GITHUB_TOKEN` or `GITHUB_TOKEN` in the environment. Pushes do not yet
support force updates, tags, or multiple refs in one operation.

## Project Status

VSTodo is under active development. Its local version-control workflows and
initial GitHub-compatible HTTP transport are implemented; broader protocol and
interoperability coverage is still in progress.

## Development Philosophy

- Build from the fundamentals.
- Prioritize correctness and reliability.
- Maintain clean, understandable code.
- Test features as they are implemented.
- Expand incrementally.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines and pull request instructions.

## License
VSTodo is licensed under the [MIT License](LICENSE).
---
**VSTodo — Version control, built differently.**


