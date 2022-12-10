# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),

## [Unreleased]

## [v0.7.0] - 2022-12-10

<!-- markdownlint-disable MD024 -->
### Added

- ENI support. You can now search for ENIs by id, tag, instance id, availability zone, private ips and public ips. [#20](https://github.com/dyegoe/awss/issues/20)
  - It is implemented without sorting support.

## [v0.6.0] - 2022-12-10

Although this is a minor version, it is a major change. The code was refactored to be more readable and maintainable. It is possible to add new features more easily. There is no binaries for this release. You can build it from source. Changes are carried over the next releases.

<!-- markdownlint-disable MD024 -->
### Changed

- General refactoring. The code is now more readable and maintainable. [#40](https://github.com/dyegoe/awss/issues/40)
- `--output json:pretty` is now `--output json-pretty`
- Configuration `show-empty` is now `show.empty`

<!-- markdownlint-disable MD024 -->  
### Removed

- Config short flag `-c`. Use `--config` instead.
- Table styles support. `--output table:style`.
- Configuration to define key/value and list separators. It uses the default only. `": "` and `"\n`.
- `--show-tags` as a flag from subcommands. Use `--show-tags` as a persistent flag instead.

<!-- markdownlint-disable MD024 -->
### Added

- `--show-tags` as a persistent flag. It will show column Tag on any table output if it exists.
- `show.tags` to the configuration file.
- `--tag-key` as a new filter for ec2 instances. It will filter instances by tag key.

## [v0.5.7] - 2022-12-02

<!-- markdownlint-disable MD024 -->
### Fixed

- Fix private and public ip filters. They were stricted to the primary ip. Now it is possible to search for any attached IP.

## [v0.5.6] - 2022-11-25

<!-- markdownlint-disable MD024 -->
### Changed

- Change function `mapToString` to accept 2 additional parameters: `kvSep` and `listSep` to allow customizing the separator between key and value and the separator between key-value pairs.
- Rename config file option from `sort_by` to `sort` to be consistent with the command line option.
- Rename config file option from `show_empty` to `show-empty` to be consistent with the command line option.
- Rename config file option from `all_regions` to `all-regions` to be consistent with the other options.
- Move config file option from `table_style` to `table.style`.
- Tags are now sorted by default when using `--output table`.

<!-- markdownlint-disable MD024 -->
### Added

- Add viper config for the `mapToString` parameters. You may add `separators.kv` and `separators.list` to the config file.

## [v0.5.5] - 2022-11-13

<!-- markdownlint-disable MD024 -->
### Fixed

- Fix `json:pretty` output format that was printing a single line output plus pretty json

## [v0.5.4] - 2022-11-13

<!-- markdownlint-disable MD024 -->
### Added

- Add EC2 sorting. You can sort the results by instance id, name, type, state, az, private ip and public ip. `--sort` flag.
  - You can change the default sorting field in the configuration file. `ec2.sort_by` field.
- Add EC2 `--show-tags` flag to show the tags in the output. (default: false)
  - You can change the default `--show-tags` value in the configuration file. `ec2.show_tags` field.

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
