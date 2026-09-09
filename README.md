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
- Restrict which tag keys are shown in table output: `--show-tags-keys Name,Environment` (implies `--show-tags`)
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
| `--volume-ids` | `-v` | Attached EBS volume IDs |
| `--cidrs` | `-c` | CIDR block of the subnet, or of the VPC when no subnet matches (`10.0.1.0/24`) |

Sort by: `--sort id|name|type|az|state|private-ip|public-ip|enis|volumes` (default: `name`)

The output also lists the EBS volumes attached to each instance.

`--cidrs` resolves the CIDR per profile and region: it looks for subnets whose CIDR block matches
exactly and searches instances in them; if none matches, it looks for VPCs with that CIDR block
associated. If neither matches, the region reports an error and no instance is returned.

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

#### VPCs (`awss vpc`)

Filter by:

| Flag | Short | Description |
| --- | --- | --- |
| `--all` | `-a` | Search all VPCs (no filters) |
| `--ids` | `-i` | VPC IDs |
| `--names` | `-n` | VPC names (tag:Name) |
| `--tags` | `-t` | Tags (`Key=Value1:Value2`) |
| `--tags-key` | `-k` | Tag keys |
| `--cidrs` | `-c` | Associated IPv4 CIDR blocks, exact match (`10.0.0.0/16`) |
| `--states` | `-s` | VPC state (`pending`, `available`) |
| `--default` | `-d` | Default VPC (`true`, `false`) |
| `--owner-ids` | `-o` | Owner account IDs |

Sort by: `--sort id|name|cidr|cidrs|state|default|owner|dhcp` (default: `name`)

#### Subnets (`awss subnet`)

Filter by:

| Flag | Short | Description |
| --- | --- | --- |
| `--all` | `-a` | Search all subnets (no filters) |
| `--ids` | `-i` | Subnet IDs |
| `--names` | `-n` | Subnet names (tag:Name) |
| `--tags` | `-t` | Tags (`Key=Value1:Value2`) |
| `--tags-key` | `-k` | Tag keys |
| `--vpc-ids` | `-V` | VPC IDs |
| `--cidrs` | `-c` | IPv4 CIDR block, exact match (`10.0.1.0/24`) |
| `--availability-zones` | `-z` | Availability zones (letter only, e.g. `a,b`) |
| `--states` | `-s` | Subnet state (`pending`, `available`) |
| `--default-for-az` | `-d` | Default subnet of its AZ (`true`, `false`) |
| `--public-ip-on-launch` | `-p` | Instances get a public IP on launch (`true`, `false`) |

Sort by: `--sort id|name|vpc-id|cidr|az|available-ips|state|public-ip|default|owner` (default: `name`)

#### S3 buckets (`awss s3`)

Buckets are listed per region, so use `--regions all` to search every region.

Filter by:

| Flag | Short | Description |
| --- | --- | --- |
| `--all` | `-a` | List all buckets (no filters) |
| `--names` | `-n` | Name patterns, globs by default (`prod-*,*-logs`) |

Sort by: `--sort name|region|created|arn` (default: `name`)

Additional flags:

- `--regex` -- treat `--names` patterns as Go regular expressions instead of globs

Name matching happens client-side (S3 has no server-side name filter). Globs are anchored:
`prod-*` matches `prod-logs` but not `my-prod-logs`; use `--regex` with `prod-` for a substring match.
Bucket tags are not shown.

#### S3 objects (`awss s3obj`)

Searches object keys inside given buckets. Each region only scans the buckets that live in it,
so use `--regions all` when you do not know the bucket's region.

| Flag | Short | Description |
| --- | --- | --- |
| `--buckets` | `-b` | Buckets to scan, exact names. Required |
| `--keys` | `-K` | Key patterns, globs by default (`logs/2024/*.gz`). Without it every key is listed |

Sort by: `--sort bucket|key|size|modified|class` (default: `key`)

Additional flags:

- `--regex` -- treat `--keys` patterns as Go regular expressions instead of globs
- `--max-keys` -- stop after scanning this many keys per bucket (default 10000) and report it

In globs `*` also matches `/`. With one pattern, its literal prefix (`logs/2024/` above) is sent
to S3 so only that part of the bucket is listed.

### Common behavior

- Filters can be combined: `awss ec2 -n '*' -s running -z a,b`
- `--all` cannot be combined with any filter flag
- Wildcard `*` matches all values in a filter
- Tags format: `Key=Value1:Value2,AnotherKey=Value`

## Installation

Download the binary for your platform (Linux and macOS, amd64 and arm64) from the
[releases](https://github.com/dyegoe/awss/releases) page and put it on your `PATH`.

Or install with Go (the version reported by `awss --version` will be `dev`):

```bash
go install github.com/dyegoe/awss@latest
```

Or build from source, which injects the version from the git tag:

```bash
git clone https://github.com/dyegoe/awss.git
cd awss
make build
cp awss /usr/local/bin
```

## Requirements

- AWS credentials. awss uses the AWS SDK's standard resolution: named profiles from
  `~/.aws/config` (`--profiles`), `AWS_PROFILE`, or `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`.
  SSO profiles work after `aws sso login`.
- Read-only IAM permissions. awss never modifies anything. The minimum policy is:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "sts:GetCallerIdentity",
        "ec2:DescribeInstances",
        "ec2:DescribeNetworkInterfaces",
        "ec2:DescribeVolumes",
        "ec2:DescribeVpcs",
        "ec2:DescribeSubnets",
        "s3:ListAllMyBuckets",
        "s3:ListBucket"
      ],
      "Resource": "*"
    }
  ]
}
```

`ec2:Describe*` is only needed for the commands you use, and `s3:ListBucket` only for `awss s3obj`.

## Configuration

AWSS uses a YAML configuration file to set defaults. The default path is `~/.awss/config.yaml`, overridable with `--config`.

```yaml
profiles:
  - default
regions:
  - us-east-1
output: table
show:
  empty: false        # --show-empty
  tags: false         # --show-tags
  tags.keys: []       # --show-tags-keys; non-empty implies show.tags
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
  no-instance-name: false
ebs:
  sort: id
  no-instance-name: false
vpc:
  sort: name
subnet:
  sort: name
s3:
  sort: name
  regex: false
s3obj:
  sort: key
  regex: false
  max-keys: 10000
```

Every key mirrors a flag: the flag wins when both are set. `all-regions` is the list
`--regions all` expands to.

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
  --show-tags-keys Name,Environment

# List all ENIs, skip instance name lookup for speed
awss eni --all --no-instance-name

# Search EBS volumes attached to a specific instance
awss ebs --instance-ids i-1234567890abcdef0

# Find the instance an EBS volume is attached to
awss ec2 --volume-ids vol-1234567890abcdef0

# Find the running instances in the subnet (or VPC) that owns a CIDR block
awss ec2 --cidrs 10.0.1.0/24 --instance-states running

# Find the VPC that owns a CIDR block
awss vpc --cidrs 10.0.0.0/16

# List the subnets of a VPC in two availability zones, fewest free IPs first
awss subnet --vpc-ids vpc-1234567890abcdef0 -z a,b --sort available-ips

# Buckets named like prod-* in every region
awss --regions all s3 --names 'prod-*'

# Buckets whose name contains "logs" or "backup", as a regular expression
awss s3 --names 'logs|backup' --regex

# Gzipped logs of January 2024 in a bucket, biggest first
awss --regions all s3obj -b my-logs -K 'logs/2024-01/*.gz' --sort size

# JSON output for scripting
awss ec2 --all --output json
```

## Contributing

Contributions are welcome! For major changes, please open an issue first.

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup and guidelines.

## License

[Apache 2.0](LICENSE)

## Dependencies

- [AWS SDK Go v2](https://github.com/aws/aws-sdk-go-v2)
- [Cobra](https://github.com/spf13/cobra)
- [Viper](https://github.com/spf13/viper)
- [Go-Pretty](https://github.com/jedib0t/go-pretty)
- [ini.v1](https://github.com/go-ini/ini) (reads `~/.aws/config` for `--profiles all`)
- [golang.org/x/term](https://pkg.go.dev/golang.org/x/term) (terminal width for tables)
