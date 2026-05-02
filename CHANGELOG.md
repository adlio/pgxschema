# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Migrated CI from CircleCI to GitHub Actions
- Upgraded golangci-lint to v2.11.4 (config migrated to v2 format)
- Added automated release workflow

## [1.0.2] - 2022-06-19

### Fixed

- Security update for `opencontainers/runc` dependency

## [1.0.1] - 2022-06-19

### Fixed

- Security updates for `opencontainers/runc` and other dependencies
- README updates

## [1.0.0] - 2022-01-04

### Added

- `FSMigrations` helper for `embed.FS` support (Go 1.16+)
- `pgxmock`-based testing infrastructure

### Changed

- Refactored `Apply` around `Begin`/`Commit` transaction flow
- Consolidated around a single connection interface
- Bumped `go.mod` to Go 1.17

## [0.0.3] - 2021-12-10

### Fixed

- Updated `jackc/pgx` and `opencontainers/runc` dependencies

## [0.0.2] - 2021-11-18

### Fixed

- Security updates to upstream dependencies

## [0.0.1] - 2021-10-07

### Added

- Initial release with PostgreSQL schema migration support
- `Migrator` with advisory locking for safe concurrent migrations
- Migration tracking via `schema_migrations` table
- Support for `pgx.Conn` and `pgxpool.Pool` connections
- File-based and directory-based migration loading
- Configurable table names and schemas

[Unreleased]: https://github.com/adlio/pgxschema/compare/v1.0.2...HEAD
[1.0.2]: https://github.com/adlio/pgxschema/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/adlio/pgxschema/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/adlio/pgxschema/compare/v0.0.3...v1.0.0
[0.0.3]: https://github.com/adlio/pgxschema/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/adlio/pgxschema/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/adlio/pgxschema/releases/tag/v0.0.1
