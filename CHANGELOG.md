# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),

## [Unreleased]

<!-- markdownlint-disable MD024 -->
### Added

- Add EC2 sorting. You can sort the results by instance id, name, type, state, az, private ip and public ip. `--sort` or `-S` flag.
- You can change the default sorting field in the configuration file. `sort_ec2_by` field.
- Add EC2 `--show-tags` flag to show the tags in the output.

<!-- markdownlint-disable MD024 -->
### Changed

- Added `header` tag to the struct fields to be able to change the header name in the output table.

## [v0.5.3] - 2022-11-13

<!-- markdownlint-disable MD024 -->
### Added

- Support for different table styles. `--output table:ascii` or `--output table:unicode` (default)
- You can set default table style in the config file. `table_style: uc`

<!-- markdownlint-disable MD024 -->
### Changed

- `--output json-pretty` is now `--output json:pretty`

## [v0.5.2] - 2022-11-10

<!-- markdownlint-disable MD024 -->
### Changed

- `-z` or `--availability-zones` accepts **only** letters to represent the availability zone. It will be append to the current region. For example, if you are searching in `us-east-1` and you want to search in `us-east-1a` you can use `-z a`.

## [v0.5.1] - 2022-10-24

<!-- markdownlint-disable MD024 -->
### Changed

- Move print function to a dedicated goroutine and add communication from search to print function via a channel
- Move error to struct field

<!-- markdownlint-disable MD024 -->
### Fixed

- Fix the bug where results are overlapping when using multiple threads
- Fix function comments

## [v0.5.0] - 2022-10-24

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
- `--profile` flag is deprecated, use `--profiles` instead.
- `--region` flag is depracated, use `--regions` instead.
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
