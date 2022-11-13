# AWSS

AWSS (stands for AWS Search) is a CLI tool to make your life easier when searching AWS resources.
It is a wrapper written in Go using AWS SDK Go v2. The work is still in progress and will be updated regularly.

## Version

<!-- Do not forget to update version on commands/commands.go Version -->
The current version is 0.5.3

## Features

The search runs in parallel using goroutines.

- Search AWS ec2 instances
  - by instance ids
  - by names
  - by tags
  - by instance types
  - by availability zones
  - by instance states
  - by private ips
  - by public ips

And you can combine these filters together.

For each search command, you can use:

- `--show-empty` to show empty results in the output
- `--sort` or `-S` to sort the results by a specific field.

## Installation

```txt
git clone http://github.com/dyegoe/awss
cd awss
go build
cp awss /usr/local/bin
```

## Configuration

AWSS uses a configuration file to set the default:

- profiles
- regions
- output format
- show empty results
- all regions to search

You can create a configuration file on your home directory `~/.awss/config.yaml` or use the `--config` flag to specify a config file.

```yaml
profiles:
  - default
regions:
  - us-east-1
output: table
show-empty: false
table_style: uc
all_regions:
  - eu-central-1
  - eu-north-1
  - eu-west-1
  - eu-west-2
  - eu-west-3
  - us-east-1
  - us-east-2
  - us-west-1
  - us-west-2
  - ca-central-1
  - sa-east-1
  - ap-south-1
  - ap-southeast-1
  - ap-southeast-2
  - ap-northeast-3
  - ap-northeast-2
  - ap-northeast-1
sort_ec2_by: name
ec2:
  show_tags: false
```

### Table Style

It implements the following table styles from `github.com/markkurossi/tabulate`:

- plain
- ascii
- uc
- uclight
- ucbolt
- compactuc
- compactuclight
- compactucbold
- colon
- simple
- simpleuc
- simpleucbold
- github
- csv

## Usage

```txt
awss --help
awss ec2 --help
awss --profiles <profile1,profile2> --regions <eu-central-1,us-east-1> ec2 --name <name>
```

### EC2

- `--show-tags` to show tags in the output. Default is `false`. It works only with `--output table`. For other outputs, it is ignored and it will shows anyway.
  
```txt
awss ec2 --help
```

## Contributing

Contributions are welcome, and they are greatly appreciated! Every little bit helps, and credit will always be given. For major changes, please open an issue first to discuss what you would like to change.

More details in [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache 2.0

## Thanks

- [AWS SDK Go v2](https://github.com/aws/aws-sdk-go-v2)
- [Cobra](https://github.com/spf13/cobra)
- [Viper](https://github.com/spf13/viper)
- [Go release binaries](https://github.com/marketplace/actions/go-release-binaries)
