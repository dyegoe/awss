# AWSS

AWSS (AWS Search) is a CLI tool that searches AWS resources in parallel across multiple profiles and regions.

Built in Go with AWS SDK Go v2, Cobra, and Viper.

## Features

- Parallel search across profiles and regions
- Multiple AWS profiles: `--profiles default,dev` or `--profiles all` (scoped by `all-profiles` in the config file)
- Multiple regions: `--regions us-east-1,eu-west-1` or `--regions all`
- Output formats: `--output table` (default), `--output json`, `--output json-pretty`
- Show empty results: `--show-empty`
- Show tags in table output: `--show-tags`
- Restrict which tag keys are shown in table output: `--show-tags-keys Name,Environment` (implies `--show-tags`)
- Timeout: `--timeout 90s` (default `5m`, `0` disables it); see [Common behavior](#common-behavior)
- Concurrency: `--concurrency 8` (default `32`), how many profile and region searches run at once
- Run stats: `--stats` prints elapsed time, peak memory, search counts and AWS API calls on stderr; see [Run report and stats](#run-report-and-stats)
- Account names next to owner account IDs, from the `accounts` map of the config file or, with `--org-profile`, from AWS Organizations; see [Account names](#account-names)
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
| `--cidrs` | `-c` | IPv4 ranges: instances with a private IP inside them (`100.64.0.0/16`) |

Sort by: `--sort id|name|type|az|state|private-ip|public-ip|enis|volumes` (default: `name`)

The output also lists the EBS volumes attached to each instance.

`--cidrs` finds the instances that have a private IP inside the range, on any network interface
(primary or secondary, and secondary IPs of an interface count too). The range can be a subnet, a
VPC CIDR block, primary or secondary, or any other IPv4 range, larger or smaller than a subnet.
Wildcards and IPv6 are not accepted.

For example, in a VPC with the CIDR blocks `10.121.224.0/20` and `100.64.0.0/16`,
`--cidrs 100.64.0.0/16` returns only the instances with an interface in the `100.64.0.0/16` subnets,
not every instance of the VPC. The Private IP column still shows the primary interface's address;
the ENIs column lists every interface.

How it works, per profile and region: awss lists the subnets and keeps those that overlap the
range, searches the instances with an interface in them (in their VPCs instead when more than 200
subnets overlap), then keeps the instances with a private IP inside the range. If no subnet
overlaps, the region reports an error and no instance is returned.

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
| `--owner-ids` | `-o` | Owner account IDs |

Sort by: `--sort id|type|az|status|subnet-id|instance-id|instance-name|owner|owner-name` (default: `id`)

Additional flags:

- `--no-instance-name` -- skip instance name lookup for faster results

The table shows the owner account of each ENI, with its name (see
[Account names](#account-names)), which tells ENIs of other accounts apart in a shared VPC.
The JSON output also carries `requester_id` and `requester_managed`, which identify
ENIs created by AWS services (Lambda, EKS, VPC endpoints, NAT gateways). Instance names of ENIs
owned by another account stay empty: look them up with that account's profile.

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

Sort by: `--sort id|name|cidr|cidrs|state|default|owner|owner-name|dhcp` (default: `name`)

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

Sort by: `--sort id|name|vpc-id|cidr|az|available-ips|state|public-ip|default|owner|owner-name` (default: `name`)

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

Bucket tags cost one extra API call per bucket, so they are fetched only when `--show-tags` or
`--show-tags-keys` is set, also for JSON output (`awss s3 -a --show-tags --output json`). A bucket you
cannot read the tags of still shows up, with the error listed above its table.

#### S3 objects (`awss s3obj`)

Searches object keys inside given buckets. `--buckets` takes bucket names or glob patterns
(`'prod-logs-*'`); each region only scans the matching buckets that live in it, so use
`--regions all` when you do not know the bucket's region.

| Flag | Short | Description |
| --- | --- | --- |
| `--buckets` | `-b` | Buckets to scan: names or glob patterns (`'prod-logs-*'`). Required |
| `--keys` | `-K` | Key patterns, globs by default (`logs/2024/*.gz`). Without it every key is listed |

Sort by: `--sort bucket|key|size|modified|class` (default: `key`)

Additional flags:

- `--regex` -- treat `--keys` patterns as Go regular expressions instead of globs
- `--max-keys` -- stop after scanning this many keys per bucket (default 10000) and report it
- `--max-buckets` -- the most buckets the patterns may match in one region (default 20). Above it,
  that region scans nothing and reports how many matched: narrow the patterns or raise the limit

In globs `*` also matches `/`. With one pattern, its literal prefix (`logs/2024/` above) is sent
to S3 so only that part of the bucket is listed. Bucket patterns are always globs: `--regex`
applies to `--keys` only.

There is no `--all`. Listing every bucket of every account is an inventory job, and each bucket
costs up to `--max-keys` / 1000 calls; use S3 Inventory with Athena, or S3 Storage Lens, for it
([#152](https://github.com/dyegoe/awss/issues/152)).

#### RDS DB instances (`awss rds`)

Searches RDS DB instances, including the instances of Aurora clusters (with their cluster ID).
`DescribeDBInstances` filters only by identifier, engine and cluster; awss matches the other
filters on the results. Filters combine with AND.

| Flag | Short | Matched by | Description |
| --- | --- | --- | --- |
| `--all` | `-a` | | Search all DB instances (no filters) |
| `--ids` | `-i` | AWS | DB instance identifiers |
| `--engines` | `-e` | AWS | Engines: `postgres`, `aurora-postgresql`, `mysql`, ... |
| `--clusters` | `-c` | AWS | The Aurora or Multi-AZ clusters the instances belong to |
| `--names` | `-n` | awss | Identifier patterns: globs, or Go regular expressions with `--regex` |
| `--engine-versions` | `-V` | awss | Engine version globs: `'13*'` finds every 13.x |
| `--tags` | `-t` | awss | `Key=Value1:Value2,Other=Value`: every key must match (AND), the values of one key are alternatives (OR), values accept globs |

```bash
# PostgreSQL 13: the engine narrows the call, the version is matched on the results
awss rds -e postgres -V '13*' --profiles all --regions all

# The instances of one Aurora cluster
awss rds -c my-aurora-cluster
```

Table columns: ID, Engine, Version, Class, Status, Multi-AZ, AZ, Cluster, and Tags with
`--show-tags` / `--show-tags-keys` (the tags come with the listing: no extra call). The table
stays about 120 characters wide, so these fields are in `--output json` only: `endpoint`
(`host:port`, empty while the instance is being created), `vpc_id`, `publicly_accessible`,
`encrypted`, `storage_type` and `storage_gib`. The endpoint is not a filter (AWS has none); find
one with jq:

```bash
awss rds -a --output json | jq -r '.data[] | [.id, .endpoint] | @tsv'
```

Sort by: `--sort id|engine|version|class|status|az|vpc|cluster` (default: `id`). Versions sort
by number: 8.0.39, 13.4, 13.15, 16.4.

#### Organization accounts (`awss org`)

Lists the accounts of the AWS Organization: ID, name, email, status and the date each account
joined.

| Flag | Short | Description |
| --- | --- | --- |
| `--statuses` | `-s` | Keep the accounts with these statuses: `active`, `suspended`, `pending-activation`, `pending-closure`, `closed` (any case; `PENDING_CLOSURE` works too) |

```bash
awss org --profiles org-management
awss org --profiles org-management --statuses suspended,pending-closure
awss org --profiles org-management --show-tags-keys Owner,CostCenter
```

`ListAccounts` has no server-side filter, so `--statuses` is matched by awss after the listing.
Account tags need one `ListTagsForResource` call per account (there is no batch call), so they
are fetched only with `--show-tags` or `--show-tags-keys`; this also adds the `tags` field to
JSON output. AWS allows 10 such calls per second per account and 12 for the whole organization,
shared with every other caller, so awss paces them at 5 per second (about 30 seconds for 150
accounts) and retries a throttled call up to 10 times. An account whose tags still cannot be read
shows its error and no tags.

- **Permission:** the profile must be allowed to call `organizations:ListAccounts`, and
  `organizations:ListTagsForResource` for tags: the management account or a delegated
  administrator. A profile without it prints the
  `AccessDeniedException` (or `AWSOrganizationsNotInUseException`) in its result set, never an
  empty list.
- **One profile:** Organizations is global and one call lists the whole organization, so `org`
  takes exactly one profile (`--profiles`, or the default resolution). Several profiles, or
  `--profiles all` with more than one, is an error before any call.
- **No region:** `--regions` is ignored; the result set shows the region `global`.
- `--output`, `--show-empty`, `--timeout` and `--stats` work as for the other commands.

Sort by: `--sort id|name|email|status|joined` (default: `name`)

### Common behavior

- Filters can be combined: `awss ec2 -n '*' -s running -z a,b`
- `--all` cannot be combined with any filter flag
- Wildcard `*` matches all values in a filter
- Tags format: `Key=Value1:Value2,AnotherKey=Value`
- `--timeout` (default `5m`) bounds the whole run. A profile and region still searching at the
  deadline is printed with the error `search timed out after <duration>` and no rows, since its
  rows could be incomplete; the profiles and regions that finished are printed as usual, and the
  command still exits 0. Use a duration with a unit (`90s`, `5m`); `0` disables the timeout. Set
  it in the config file with `timeout:`.
- `--concurrency` (default `32`) caps the profile and region searches that run at once; the
  others wait for a free slot without calling AWS. `--profiles all --regions all` over 150
  profiles is about 2,550 searches: the cap bounds the open connections, the credential lookups
  and the finished result sets waiting in memory to be printed. A search still waiting at the
  `--timeout` deadline is not started and is printed with the error
  `search did not start before the <duration> timeout`; raise `--timeout` or `--concurrency`.
  The value must be 1 or more. Set it in the config file with `concurrency:`.

### Run report and stats

When at least one search failed, awss prints one line on stderr after the results:

```text
awss: 3 of 2,550 searches failed (2 errors, 1 timed out); see the result sets marked with errors
```

"Errors" counts the result sets with an error (an expired SSO session, `AccessDenied`, ...);
"timed out" counts the ones that hit `--timeout`, including those that did not start. A clean run
prints nothing extra. The line goes to stderr, so `--output json` on stdout still parses with
`jq`, and the command still exits 0.

`--stats` (or `stats: true` in the config file) adds a summary on stderr after the results:

```text
awss stats
  elapsed        1.97s
  peak memory    146 MB
  profiles       150   regions 17   searches 2,550 (concurrency 32)
  searches       2,512 with results, 35 empty, 2 failed, 1 timed out
  resources      48,213
  API calls      2,904 (61 retries, 12 throttled)
```

- **peak memory** is the most memory the OS charged the process (`getrusage` max RSS), on Linux
  and macOS; `n/a` elsewhere.
- **searches** counts each profile and region once: `failed` has an error, `timed out` hit
  `--timeout`.
- **resources** is the rows found by the searches that did not time out.
- **API calls** counts every AWS operation called, one per call (each page of a paginated
  search is a call); **retries** are the attempts beyond the first, and **throttled** the attempts
  AWS answered with a throttling error.

### Account names

`vpc`, `subnet` and `eni` show an **Owner** column (JSON `owner_name`, sort field `owner-name`)
next to **Owner ID**: the name of the account, which matters in a shared VPC, where the owner is
usually a network account. The names come from the `accounts` map of `~/.awss/config.yaml`, from
account ID to name, with no AWS call:

```yaml
accounts:
  "123456789012": network-prd
  "012345678901": Shared-Services
```

- Quote the IDs: an unquoted ID is read as a number and loses its leading zeros (awss restores
  them, since account IDs have 12 digits).
- An entry that is not an account ID, or has no name, is skipped with a warning on stderr. An ID
  listed twice is a YAML error: the config file does not load.
- An account not in the map leaves **Owner** empty. Profile names of the AWS config file are not
  used: a profile name such as `admins-network-prd` names a role in an account, not the account.

**Names from AWS Organizations (opt-in).** `--org-profile <profile>`, or `org-profile:` in the
config file, makes `vpc`, `subnet` and `eni` call `organizations:ListAccounts` once per run with
that profile and name every account of the organization:

```bash
awss subnet --all --profiles member-account --org-profile org-management
```

- The `accounts` map wins: the organization only names the accounts the map does not.
- It is off by default: it adds a call and needs a permission most users do not have.
- If the call fails, the run goes on with the names of the `accounts` map, and one warning goes to
  stderr. The search results are unaffected. The call is bounded by `--timeout`.

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

- AWS credentials. awss uses the AWS SDK's standard resolution: named profiles from the AWS
  config file (`--profiles`), `AWS_PROFILE`, or `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`. The
  AWS config file is `AWS_CONFIG_FILE` when set, `~/.aws/config` otherwise, as for the AWS CLI;
  awss validates `--profiles` and expands `--profiles all` from that same file.
- A valid session for every profile you search. awss never logs in: run `aws sso login`, or your
  credential helper (for example Granted), before searching. A profile without a valid session
  reports its own error in its result set; the other profiles are searched as usual.
- Read-only IAM permissions. awss never modifies anything. The minimum policy is:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
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
all-profiles: []      # what --profiles all expands to; empty means every profile in the AWS config file
regions:
  - us-east-1
output: table
timeout: 5m           # --timeout; a duration with a unit (90s, 5m), 0 disables it
concurrency: 32       # --concurrency; profile and region searches running at once, 1 or more
stats: false          # --stats; print the run stats on stderr after the results
accounts: {}          # account ID -> name for the Owner column; see Account names
org-profile: ""       # --org-profile; profile that lists the organization's account names (opt-in)
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
  max-buckets: 20
```

Every key mirrors a flag: the flag wins when both are set. `all-regions` is the list
`--regions all` expands to. `all-profiles` is the list `--profiles all` expands to; when it is
empty or missing, `--profiles all` uses every `[default]` and `[profile ...]` section of the
AWS config file (`AWS_CONFIG_FILE`, or `~/.aws/config`). Set it when that file holds profiles you
do not want to search, such as several privilege variants per account or MFA source profiles.
Each entry must exist in the AWS config file.

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

# Find the running instances with a private IP in a range (a subnet, a VPC CIDR block or any range)
awss ec2 --cidrs 100.64.0.0/16 --instance-states running

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

# Every PostgreSQL 13 instance, everywhere
awss rds -e postgres -V '13*' --profiles all --regions all

# JSON output for scripting
awss ec2 --all --output json

# Every profile and region, with the run stats on stderr and the JSON results in a file
awss ec2 --all --profiles all --regions all --output json --stats > instances.json
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
- [ini.v1](https://github.com/go-ini/ini) (reads the AWS config file for `--profiles all` and profile validation)
- [golang.org/x/term](https://pkg.go.dev/golang.org/x/term) (terminal width for tables)
