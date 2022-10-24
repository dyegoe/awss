# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),

## [Unreleased]

<!-- markdownlint-disable MD024 -->
### Added

- `CHANGELOG.md` file
- `spf13/viper` dependency and enable configuration file support
- `--config` flag to specify a config file

<!-- markdownlint-disable MD024 -->
### Changed

- Package `cmd` to `commands`
- Main cobra.Command `rootCmd` to `awssCmd`
- Include configuration on `README.md`
- Because of new configuration, `--profile` flag is now `--profiles`. Same for `--region` flag that is now `--regions`.
- To simplify, `--show-empty-results` flag is now `--show-empty`

## [v0.4.0] - 2022-10-18

<!-- markdownlint-disable MD024 -->
### Added

- Support for new filters
  - by instance types
  - by availability zones
  - by instance states
- Support for search using more than one filter at a time

## [v0.3.0] - 2022-10-15

<!-- markdownlint-disable MD024 -->
### Added

- Paralellism using goroutines
- `--show-empty-results` option to show empty results in the output
- Function `printResults`

<!-- markdownlint-disable MD024 -->
### Changed

- Updated `README.md`
- Use `os.Exit(1)` instead of `panic()` when an error occurs
- General refactoring on print funcitions
- General refactoring on error handling

<!-- markdownlint-disable MD024 -->
### Removed

- `Errors` field from `instances` struct

## [v0.2.4] - 2022-10-08

<!-- markdownlint-disable MD024 -->
### Removed

- `LICENSE` and `README.md` from the package.

## [v0.2.3] - 2022-10-08

<!-- markdownlint-disable MD024 -->
### Added

- Initial release
- Search AWS ec2 instances
  - by name
  - by tag
  - by instance id
  - by private ip
  - by public ip
