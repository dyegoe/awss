# AWSS

AWSS (AWS Search) is a CLI tool that searches AWS resources in parallel across multiple profiles and regions.

Built in Go with AWS SDK Go v2, Cobra, and Viper.

## Features

- Parallel search across profiles and regions
- Multiple AWS profiles: `--profiles default,dev` or `--profiles all`
- Multiple regions: `--regions us-east-1,eu-west-1` or `--regions all`
- Output formats: `--output table` (default), `--output json`, `--output json-pretty`
- Show empty results: `--show-empty`
- Show tags in table output: `--show-tags`
- Configuration file: `--config` (default `~/.awss/config.yaml`)
- Version injected at build time via `-ldflags`

### Supported resources

#### EC2 instances (`awss ec2`)

Filter by:

| Flag | Short | Description |
| --- | --- | --- |
| `--all` | `-a` | Search all instances (no filters) |
| `--ids` | `-i` | Instance IDs |
| `--names` | `-n` | Instance names (tag:Name) |
| `--tags` | `-t` | Tags (`Key=Value1:Value2`) |
| `--tags-key` | `-k` | Tag keys |
| `--instance-types` | `-T` | Instance types |
| `--availability-zones` | `-z` | Availability zones (letter only, e.g. `a,b`) |
| `--instance-states` | `-s` | Instance states |
| `--private-ips` | `-p` | Private IP addresses |
| `--public-ips` | `-P` | Public IP addresses |

Sort by: `--sort id|name|type|az|state|private-ip|public-ip|enis` (default: `name`)

#### ENI (`awss eni`)

Filter by:

| Flag | Short | Description |
| --- | --- | --- |
| `--all` | `-a` | Search all ENIs (no filters) |
| `--ids` | `-i` | Network interface IDs |
| `--tags` | `-t` | Tags (`Key=Value1:Value2`) |
| `--tags-key` | `-k` | Tag keys |
| `--instance-ids` | `-I` | Attached instance IDs |
| `--availability-zones` | `-z` | Availability zones |
| `--private-ips` | `-p` | Private IP addresses |
| `--public-ips` | `-P` | Public IP addresses |

Sort by: `--sort id|type|az|status|subnet-id|instance-id|instance-name` (default: `id`)

Additional flags:

- `--no-instance-name` -- skip instance name lookup for faster results

#### EBS volumes (`awss ebs`)

Filter by:

| Flag | Short | Description |
| --- | --- | --- |
| `--all` | `-a` | Search all volumes (no filters) |
| `--ids` | `-i` | Volume IDs |
| `--tags` | `-t` | Tags (`Key=Value1:Value2`) |
| `--tags-key` | `-k` | Tag keys |
| `--availability-zones` | `-z` | Availability zones |
| `--statuses` | `-s` | Volume status (`available`, `in-use`, etc.) |
| `--volume-types` | `-T` | Volume types (`gp2`, `gp3`, `io1`, etc.) |
| `--instance-ids` | `-I` | Attached instance IDs |
| `--encrypted` | `-e` | Encryption status (`true`, `false`) |

Sort by: `--sort id|size|type|state|az|encrypted|instance-id|instance-name|device` (default: `id`)

Additional flags:

- `--no-instance-name` -- skip instance name lookup for faster results

### Common behavior

- Filters can be combined: `awss ec2 -n '*' -s running -z a,b`
- `--all` cannot be combined with any filter flag
- Wildcard `*` matches all values in a filter
- Tags format: `Key=Value1:Value2,AnotherKey=Value`

## Installation

Download the binary from the [releases](https://github.com/dyegoe/awss/releases) page.

Or build from source:

```bash
git clone https://github.com/dyegoe/awss.git
cd awss
make build
cp awss /usr/local/bin
```

## Configuration

AWSS uses a YAML configuration file to set defaults. The default path is `~/.awss/config.yaml`, overridable with `--config`.

```yaml
profiles:
  - default
regions:
  - us-east-1
output: table
show:
  empty: false
  tags: false
all-regions:
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
ec2:
  sort: name
eni:
  sort: id
ebs:
  sort: id
```

## Usage

```bash
# List all EC2 instances across two profiles and three regions
awss --profiles default,dev \
  --regions eu-central-1,us-east-1,sa-east-1 \
  ec2 --all

# Search EC2 instances with multiple filters
awss ec2 \
  --names 'web-*' \
  --tags 'Environment=prod' \
  --instance-states running \
  --sort name \
  --show-tags

# List all ENIs, skip instance name lookup for speed
awss eni --all --no-instance-name

# Search EBS volumes attached to a specific instance
awss ebs --instance-ids i-1234567890abcdef0

# JSON output for scripting
awss ec2 --all --output json
```

## Contributing

Contributions are welcome! For major changes, please open an issue first.

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup and guidelines.

## License

Apache 2.0

## Dependencies

- [AWS SDK Go v2](https://github.com/aws/aws-sdk-go-v2)
- [Cobra](https://github.com/spf13/cobra)
- [Viper](https://github.com/spf13/viper)
- [Go-Pretty](https://github.com/jedib0t/go-pretty)
