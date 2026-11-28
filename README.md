# AWSS

AWSS (AWS Search) is a CLI tool that searches AWS resources in parallel across multiple profiles and regions.
It also cleans up: `awss cleanup default-vpc` deletes the default VPCs that hold no workload, and
`awss cleanup dhcp-options` the DHCP options sets no VPC uses (both a preview by default).

Built in Go with AWS SDK Go v2, Cobra, and Viper.

## Features

- Parallel search across profiles and regions
- Multiple AWS profiles: `--profiles default,dev` or `--profiles all` (scoped by `all-profiles` in the config file)
- Multiple regions: `--regions us-east-1,eu-west-1` or `--regions all`
- `--profile` and `--region` work too, as in the AWS CLI: they are the same flags as `--profiles` and `--regions`
- Output formats: `--output table` (default), `--output json`, `--output json-pretty`. To keep the
  table narrow, some columns hold two or three values on separate lines, such as an ID with its name
  under it; the header cell names each line. JSON keeps one field for each.
- Field reference in `--help`: every command's help ends with a **Fields** section, one line per
  field of the result: its JSON name, its table column (or "JSON only"), its sort field and what it means
- Show empty results: `--show-empty`
- Show tags in table output: `--show-tags`
- Restrict which tag keys are shown in table output: `--show-tags-keys Name,Environment` (implies `--show-tags`)
- Timeout: `--timeout 90s` (default `5m`, `0` disables it); see [Common behavior](#common-behavior)
- Concurrency: `--concurrency 8` (default `32`), how many profile and region searches run at once
- Run stats: `--stats` prints elapsed time, peak memory, search counts and AWS API calls on stderr; see [Run report and stats](#run-report-and-stats)
- Account names next to owner account IDs, from the `accounts` map of the config file or, with `--org-profile`, from AWS Organizations; see [Account names](#account-names)
- Shell completion of commands, flags and the values of the flags that take a known list (sort fields, profiles, regions, states); see [Shell completion](#shell-completion)
- AWS Cloud WAN: find a CIDR in the route tables of the segments and see where it points (`awss cloudwan routes`), see the BGP routes before the routing policies with their AS path and communities (`awss cloudwan bgp`), and list the attachments (`awss cloudwan attachments`); see [Cloud WAN routes](#cloud-wan-routes-awss-cloudwan-routes)
- AWS Direct Connect: list the gateways, what they are associated with and the prefixes they advertise to the data centres (`awss dx gateways`); see [Direct Connect gateways](#direct-connect-gateways-awss-dx-gateways)
- AWS Transit Gateway: find a CIDR in the Transit Gateway route tables and see which attachment (VPC, VPN, Direct Connect, peering) holds it (`awss tgw routes`), and list the attachments with the route table each one is associated with (`awss tgw attachments`); see [Transit Gateway routes](#transit-gateway-routes-awss-tgw-routes) and [Transit Gateway attachments](#transit-gateway-attachments-awss-tgw-attachments)
- Cleanup of the default VPCs with no workload (`awss cleanup default-vpc`) and of the unused DHCP options sets (`awss cleanup dhcp-options`), a preview unless `--apply`; see [Cleanup](#cleanup)
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

In `--output json` only:

- `created`: an estimate of when the instance was created. EC2 has no creation time, so awss
  takes the attach time of the primary network interface, which is attached at launch and can
  never be detached. Without it, it takes the attach time of the root volume, which changes if
  the root volume is replaced.
- `launched`: the last start (`LaunchTime`). It changes on every stop and start, so it is not
  the creation time.

Both are RFC 3339 in UTC (`2021-06-02T14:10:55Z`).

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

In `--output json` only: `created`, the creation date of the volume (RFC 3339 in UTC).

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
`encrypted`, `storage_type`, `storage_gib`, `created` (the creation date, RFC 3339 in UTC, empty
while the instance is being created) and `license_model` (such as `postgresql-license` or
`license-included`). The endpoint is not a filter (AWS has none); find
one with jq:

```bash
awss rds -a --output json | jq -r '.data[] | [.id, .endpoint] | @tsv'
```

Sort by: `--sort id|engine|version|class|status|az|vpc|cluster` (default: `id`). Versions sort
by number: 8.0.39, 13.4, 13.15, 16.4.

#### RDS DB clusters (`awss rds-cluster`)

Searches RDS DB clusters: Aurora, and Multi-AZ DB clusters. `DescribeDBClusters` filters only by
identifier and engine; awss matches the other filters on the results, with the same syntax and
rules as `awss rds`.

| Flag | Short | Matched by | Description |
| --- | --- | --- | --- |
| `--all` | `-a` | | Search all DB clusters (no filters) |
| `--ids` | `-i` | AWS | DB cluster identifiers |
| `--engines` | `-e` | AWS | Engines: `aurora-postgresql`, `aurora-mysql`, `postgres`, `mysql` |
| `--names` | `-n` | awss | Identifier patterns: globs, or Go regular expressions with `--regex` |
| `--engine-versions` | `-V` | awss | Engine version globs: `'8.0.*'` |
| `--tags` | `-t` | awss | `Key=Value1:Value2,Other=Value`: every key must match (AND), the values of one key are alternatives (OR), values accept globs |

```bash
# Every Aurora PostgreSQL cluster, everywhere
awss rds-cluster -e aurora-postgresql --profiles all --regions all

# Then the instances of one of them
awss rds -c my-aurora-cluster
```

Table columns: ID, Engine, Version, Status, Members (`1 writer, 2 readers`), VPC, and Tags with
`--show-tags` / `--show-tags-keys` (no extra call). These fields are in `--output json` only, to
keep the table narrow: `writer_endpoint`, `reader_endpoint`, `port`, `members` (`[{id, writer}]`),
`multi_az`, `subnet_group`, `encrypted`, `deletion_protection` and `created` (the creation date,
RFC 3339 in UTC).

The VPC comes from the cluster's DB subnet group: one paginated `DescribeDBSubnetGroups` call per
profile and region, only when clusters were found. If that call fails, the clusters are still
listed with the error and an empty VPC.

Sort by: `--sort id|engine|version|status|vpc` (default: `id`).

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

#### CloudFront distributions (`awss cloudfront`)

Lists the CloudFront distributions of each profile: ID, CloudFront domain, aliases (CNAMEs),
origins, status and whether it is enabled.

| Flag | Short | Description |
| --- | --- | --- |
| `--all` | `-a` | Search all distributions (no filters) |
| `--ids` | `-i` | Distribution IDs, exact |
| `--domains` | `-d` | Aliases or the `*.cloudfront.net` domain: globs, or Go regular expressions with `--regex` |
| `--origins` | `-O` | Origin domain names: globs, or regular expressions with `--regex` |
| `--tags` | `-t` | Tags (`Key=Value1:Value2`); values accept globs |
| `--regex` | | Treat `--domains` and `--origins` as regular expressions |

```bash
# Which distribution serves www.example.com, in any account
awss cloudfront -d www.example.com --profiles all

# Every distribution in front of an S3 bucket
awss cloudfront -O '*.s3.amazonaws.com' --profiles all

# Production distributions, with their tags
awss cloudfront -t Env=prod --show-tags
```

`ListDistributions` cannot filter, so awss matches every filter on the results. Filters combine
with AND.

- **Global:** CloudFront is global, so `cloudfront` searches each profile once, in no region.
  `--regions` is ignored; the result set shows the region `global`. Several profiles and
  `--profiles all` work as for the other commands.
- **Tags:** the listing has no tags; each distribution needs one `ListTagsForResource` call, so
  they are fetched only with `--show-tags`, `--show-tags-keys` or `--tags`. awss paces them at 5
  per second (AWS does not publish this quota) and retries a throttled call up to 10 times. The
  tag calls run only for the distributions that passed the other filters. A distribution whose
  tags cannot be read shows its error, and does not match `--tags`.
- **JSON only:** `arn`, `comment`, `price_class`, `http_version`, `ipv6`, `web_acl_id`,
  `certificate` (the ACM certificate ARN, or the IAM certificate ID; empty for the default
  certificate), `staging` and `modified` (the last change, RFC 3339 in UTC). CloudFront returns
  no creation time.
- **Permission:** `cloudfront:ListDistributions`, and `cloudfront:ListTagsForResource` for tags.

Sort by: `--sort id|domain|status|modified` (default: `id`)

#### Cloud WAN routes (`awss cloudwan routes`)

Finds which segments and edge locations of an AWS Cloud WAN core network have a route to a CIDR,
and where each route points: a VPC, a Direct Connect gateway, a VPN, a Connect peer, a Transit
Gateway route table, or nowhere (blackhole). It answers reachability questions such as "is the
data centre prefix learned over Direct Connect in the prod segment at every edge?" without going
through the route table of every segment in the console.

| Flag | Short | Description |
| --- | --- | --- |
| `--cidrs` | `-c` | CIDRs or IP addresses to find; a bare IP is its `/32` (IPv6 too). Without it, every route of the selected route tables is listed |
| `--match` | `-m` | How a route must relate to each CIDR: `any` (default: `exact`, `subnet-of` and `supernet-of` together), `exact`, `longest` (the route traffic takes), `subnet-of` (routes inside the CIDR), `supernet-of` (routes covering it) |
| `--segments` | `-s` | Route tables of these segments only (globs) |
| `--function-groups` | | Route tables of these network function groups only (globs) |
| `--edges` | `-e` | Route tables of these edge locations only (globs) |
| `--types` | `-T` | Route types: `propagated`, `static`, `system` |
| `--states` | | Route states: `active`, `blackhole` |
| `--target-types` | | Next hop types, such as `direct-connect-gateway`, `vpc`, `vpn`, `connect-peer`, `transit-gateway-route-table`, `blackhole`; `dx`, `tgw` and `connect` for short |
| `--core-network-ids` | | Core networks to search; default every core network the profile sees |

```bash
# Is the data centre prefix learned over Direct Connect, and in which segments and edges?
awss cloudwan routes --profile network --cidrs 10.50.0.0/16 --target-types direct-connect-gateway

# Which route does traffic to 10.20.1.5 take in the prod segment at eu-west-1?
awss cloudwan routes --profile network --cidrs 10.20.1.5 --match longest -s prod -e eu-west-1

# Every route of the prod segment, by CIDR
awss cloudwan routes --profile network -s prod --sort cidr

# The blackholes of every route table
awss cloudwan routes --profile network --states blackhole
```

One row per route destination: **Query** (the CIDR searched), **Segment** (`nfg:<name>` for a
network function group), **Edge**, **CIDR**, **Type**, **State**, **Target Type** / **Target ID** /
**Attachment** (one cell: the type and ID of the next hop, and the Name tag of its attachment),
**Target Segment** (the segment of that attachment: for a route shared from another segment, the
segment it came from, such as a Direct Connect route in `dev` whose gateway is in `sharedsvc`), and
**Owner ID** / **Name** (one cell: the account of the attachment and its name; see
[Account names](#account-names)). AWS returns only the attachment, type
and resource of a route destination, so the name, segment, owner and edges come from the
attachments of the core network.

- **Profile:** run it with a profile of the account that owns the core network, usually the
  network account. An account the core network is shared with gets AccessDenied, reported once
  per core network.
- **Global:** Network Manager is global and answers in `us-west-2`, so the command searches each
  profile once, in no region; `--regions` is ignored and the result set shows the region
  `global`. Use `--edges` to pick edge locations.
- **Calls:** one `GetNetworkRoutes` call per route table (each segment and each network function
  group, at each edge location), per CIDR and per match mode (`any` is three), at most 5 at once.
  AWS has no call that reads several route tables, and does not document how several match
  fields of one call combine, so awss sends one each. `--segments`, `--edges` and `--match` cut
  the calls. `--types` and `--states` are filtered by AWS, `--target-types` by awss.
- **What it shows:** the routes Cloud WAN installed, after its routing policies. A prefix the data
  centre advertised but a policy dropped is not there: look for it with
  [`awss cloudwan bgp`](#cloud-wan-bgp-routes-awss-cloudwan-bgp).
- **Transit Gateway:** behind a Transit Gateway route table attachment, the route points to the
  Transit Gateway route table; find the VPC behind it with
  [`awss tgw routes`](#transit-gateway-routes-awss-tgw-routes).
- **JSON only:** `core_network_id`, `prefix_list_id`, `attachment_id`, `target_edge`,
  `transit_gateway_attachment_id`.
- **Permission:** `networkmanager:ListCoreNetworks`, `networkmanager:GetCoreNetwork`,
  `networkmanager:GetNetworkRoutes`, and `networkmanager:ListAttachments` for the attachment
  names and owners.

Sort by: `--sort segment|edge|cidr|query|type|state|target-type|target-id|target-segment|attachment|attachment-name|owner|owner-name|core-network` (default: `segment`). `cidr` sorts by address (`10.2.0.0/16` before `10.100.0.0/16`, a supernet before its subnets), and rows equal on the sort field stay in route table order: segment, edge, CIDR.

#### Cloud WAN BGP routes (`awss cloudwan bgp`)

Shows the BGP routes each segment learned at each edge location **before** the routing policies
of the core network, with their AS path, communities, local preference, MED and next hop, and
whether each made it into the route table (**Installed**, from `cloudwan routes`):

- `yes`: the installed route of the prefix points to this path's attachment;
- `no`: the installed route of the prefix points to another attachment, such as the backup Direct
  Connect gateway;
- `none`: no route of the prefix is installed: a routing policy dropped or summarised it.

A prefix the data centre never advertised is in neither command.

**How Installed is decided.** AWS has no installed flag on a BGP route, so awss compares two calls
for each segment and edge location, with the same CIDR and match:

1. `ListCoreNetworkRoutingInformation`: the BGP routes, every path learned for a prefix. These are
   the rows.
2. `GetNetworkRoutes`: the routes installed in the route table of that segment and edge, as
   `cloudwan routes` shows them. Each installed prefix gives the attachments its route points to.

A row is `yes` when its prefix is installed and the installed route points to the row's attachment,
`no` when the prefix is installed but points elsewhere, and `none` when the prefix is not installed.
For example, `10.120.0.0/16` learned over two Direct Connect attachments A and B and installed
through A: the path through A is `yes`, the path through B is `no`.

- **Attachments, not paths:** AWS names only the attachment of an installed route, so every path
  through that attachment (two VPN tunnels, two Connect peers, two AS paths over one Direct Connect
  gateway) is `yes`: awss does not guess the BGP winner.
- **Any installed route counts:** the type and state of the installed route are not checked. A
  static route of the prefix to the attachment of a BGP path makes that path `yes`, though the
  static route won, not BGP.
- **ECMP:** an installed route with several attachments makes the path through each of them `yes`.
- **Exact prefix:** a BGP prefix covered only by a shorter installed route, such as a summary
  `10.0.0.0/8`, is `none`. So is a prefix installed only through a prefix list route, which has no
  CIDR. With `--match longest`, the most specific BGP prefix can be `none` while the traffic takes
  a shorter installed route: `cloudwan routes --match longest` shows which.
- **No attachment:** a path is never `yes` without an attachment. An installed route of the prefix
  that points to no attachment, such as a static blackhole, makes every path of the prefix `no`.
- **Every next hop counts:** `--next-hop-types` filters the rows, not the installed routes. A
  prefix installed through a VPC makes a Direct Connect path of the same prefix `no`, even with
  `--next-hop-types dx`.
- **Errors:** when `GetNetworkRoutes` fails, the BGP routes of that segment and edge are not shown,
  and the error is in the result set.

| Flag | Short | Description |
| --- | --- | --- |
| `--cidrs` | `-c` | CIDRs or IP addresses to find, as in `cloudwan routes`. Without it, every BGP route of the selected segments is listed |
| `--match` | `-m` | `any` (default), `exact`, `longest`, `subnet-of`, `supernet-of`, as in `cloudwan routes` |
| `--segments` | `-s` | Segments only (globs) |
| `--edges` | `-e` | Edge locations only (globs) |
| `--next-hop-types` | | Next hop types, such as `direct-connect-gateway`, `vpc`, `vpn`, `connect-peer`; `dx`, `tgw` and `connect` for short |
| `--communities` | | Keep the routes that carry any of these BGP communities, exact (`65000:100`) |
| `--asns` | | Keep the routes whose AS path has any of these AS numbers (`65010`) |
| `--installed` | | Keep the routes with these **Installed** values: `yes`, `no`, `none` |
| `--core-network-ids` | | Core networks to search; default every core network the profile sees |

```bash
# What does the data centre advertise over Direct Connect to the prod segment?
awss cloudwan bgp --profile network -s prod --next-hop-types dx

# What does the data centre advertise that never reaches the prod route table?
awss cloudwan bgp --profile network -s prod --installed none

# Did 10.50.0.0/16 arrive, with which AS path, and is it also installed?
awss cloudwan bgp --profile network --cidrs 10.50.0.0/16
awss cloudwan routes --profile network --cidrs 10.50.0.0/16

# The routes originated by the data centre AS 65010
awss cloudwan bgp --profile network --asns 65010
```

One row per BGP route: **Query**, **Segment** / **Edge** (one cell), **Prefix** / **Installed** (one
cell), **AS Path**, **Communities**, **Local Pref**, **MED**, **Next Hop Type** / **Next Hop ID** /
**Attachment** / **Next Hop Segment** (one cell; the segment comes from the attachment when AWS does not
return it) and **Owner ID** / **Name** (one cell).

- One `ListCoreNetworkRoutingInformation` call per segment, edge location, CIDR and match mode, at
  most 5 at once, and one `GetNetworkRoutes` call with the same match for **Installed** (skipped
  when there is no BGP route). Network function groups have no BGP routes. `longest` reads every route of the
  segment and awss picks the most specific one, as in `cloudwan routes`. `--next-hop-types`,
  `--communities`, `--asns` and `--installed` are filtered by awss.
- **What it shows:** every candidate path, so a prefix advertised over several attachments (two
  Direct Connect gateways, many Transit Gateway route tables) has one row per path, and a segment
  can have many more rows here than in `cloudwan routes`. VPC routes have no AS path, local
  preference or MED. Narrow it with `--cidrs`, `--next-hop-types` or `--asns`.
- Global, and run with the network account profile, as `cloudwan routes`.
- **JSON only:** `core_network_id`, `next_hop_ip` (such as the BGP peer of a Connect peer),
  `attachment_id`, `next_hop_edge`.
- **Permission:** `networkmanager:ListCoreNetworks`, `networkmanager:GetCoreNetwork`,
  `networkmanager:ListCoreNetworkRoutingInformation`, `networkmanager:GetNetworkRoutes`, and
  `networkmanager:ListAttachments` for the attachment names and owners.

Sort by: `--sort segment|edge|prefix|installed|query|as-path|communities|local-preference|med|next-hop-type|next-hop-id|next-hop-ip|next-hop-segment|attachment|attachment-name|owner|owner-name|core-network` (default: `segment`). `installed`
sorts `yes`, `no`, then `none`. Rows equal on the sort field stay in route table order: segment,
edge, prefix by address, the installed path first.

#### Cloud WAN attachments (`awss cloudwan attachments`)

Lists the attachments of the Cloud WAN core networks: which VPC, Direct Connect gateway,
Site-to-Site VPN, Connect or Transit Gateway route table is attached to which segment, at which
edge locations, owned by which account.

| Flag | Short | Description |
| --- | --- | --- |
| `--ids` | `-i` | Attachment IDs, exact |
| `--names` | `-n` | Name tags (globs) |
| `--types` | `-T` | `vpc`, `direct-connect-gateway`, `site-to-site-vpn`, `connect`, `transit-gateway-route-table` |
| `--segments` | `-s` | Segments, or `nfg:<name>` for a network function group (globs) |
| `--edges` | `-e` | Edge locations (globs); a Direct Connect gateway spans several and matches when any does |
| `--states` | | States, such as `available`, `pending-attachment-acceptance`, `failed` |
| `--core-network-ids` | | Core networks to list; default every core network the profile sees |

```bash
# The Direct Connect gateways, and the segments and edges they serve
awss cloudwan attachments --profile network --types direct-connect-gateway

# What is attached to the prod segment, with the account names of the organization
awss cloudwan attachments --profile network -s prod --org-profile org-management

# Attachments waiting for acceptance or failed
awss cloudwan attachments --profile network --states pending-attachment-acceptance,failed
```

Columns: **ID** / **Name** (one cell), **Type**, **Segment**, **Edges**, **State**, **Resource** (the
ID of the attached resource, from its ARN), **Owner ID** / **Name** (one cell) and, with
`--show-tags`, **Tags**.

- `ListAttachments` filters on one value at most, so awss lists every attachment of the core
  network and matches the filters on the results. Filters combine with AND.
- Global, and run with the network account profile, as `cloudwan routes`.
- **JSON only:** `core_network_id`, `resource_arn`, `policy_rule` (the attachment policy rule that
  placed it in its segment) and `created` (RFC 3339 in UTC).
- **Permission:** `networkmanager:ListCoreNetworks` and `networkmanager:ListAttachments`.

Sort by: `--sort segment|id|name|type|edge|state|resource|owner|owner-name|created|core-network` (default: `segment`)

#### Direct Connect gateways (`awss dx gateways`)

Lists the Direct Connect gateways, what each one is associated with (a transit gateway, a virtual
private gateway or a Cloud WAN core network), the prefixes it advertises to the data centres, and
the virtual interfaces (VIFs) attached to it: the BGP sessions to the data centres.

The allowed prefixes of an association to a transit gateway or a virtual private gateway are what
the data centres learn over Direct Connect from that gateway. An association to a Cloud WAN core
network has none: Cloud WAN advertises the routes of the segment the gateway is attached to, which
`awss cloudwan routes` shows.

| Flag | Short | Description |
| --- | --- | --- |
| `--ids` | `-i` | Gateway IDs, exact |
| `--names` | `-n` | Gateway names (globs) |
| `--cidrs` | `-c` | CIDRs or IP addresses to find in the allowed prefixes; a bare IP is its /32 |
| `--match` | `-m` | How an allowed prefix must relate to each CIDR: `any` (default: exact, subnet-of and supernet-of), `exact`, `subnet-of` (inside the CIDR), `supernet-of` (covering it: the prefixes that carry an address) |
| `--associated-types` | | `core-network`, `transit-gateway`, `virtual-private-gateway` |
| `--associated-ids` | | Transit gateway, virtual private gateway or core network IDs, exact |
| `--states` | | Association states: `associating`, `associated`, `disassociating`, `disassociated`, `updating` |

```bash
# Is 10.20.1.5 advertised to the data centres, and by which gateway and association?
awss dx gateways --profile network --cidrs 10.20.1.5 --match supernet-of

# The gateways attached to Cloud WAN, and their VIFs
awss dx gateways --profile network --associated-types core-network

# Everything one transit gateway advertises over Direct Connect, in address order
awss dx gateways --profile network --associated-ids tgw-0123456789abcdef0 --sort prefix
```

Columns: **Query**, **Gateway ID** / **Name** (one cell), **Amazon ASN**, **Associated Type**,
**Associated ID**, **Region** (of the associated gateway), **Owner ID** / **Name** (one cell),
**State** (of the association), **Allowed Prefix** and **VIFs**.

- One row per allowed prefix of each association, or per allowed prefix and searched CIDR with
  `--cidrs`. An association without allowed prefixes, and a gateway without associations, is one
  row with those columns empty; `--cidrs` and the association filters leave them out.
- Every filter is matched by awss on the results. Filters combine with AND.
- A Direct Connect gateway is global: the command searches each profile once, in the region of the
  profile (us-east-1 when it names none), and ignores `--regions`. Run it with a profile of the
  account that owns the gateways, usually the network account.
- AWS lists the associations and the VIFs of one gateway per call, so the command makes two calls
  per gateway.
- **JSON only:** `gateway_state`, `gateway_owner_id`, `association_id`,
  `core_network_attachment_id` (the Cloud WAN attachment of the gateway), `state_change_error` and
  `vif_attachments` (each VIF with its region, owner and state).
- **Permission:** `directconnect:DescribeDirectConnectGateways`,
  `directconnect:DescribeDirectConnectGatewayAssociations` and
  `directconnect:DescribeDirectConnectGatewayAttachments`.

Sort by: `--sort gateway|gateway-id|amazon-asn|associated-type|associated-id|region|owner|owner-name|state|prefix|query` (default: `gateway`). Rows equal
on the sort field stay grouped by gateway and association, the prefixes in address order.

#### Transit Gateway routes (`awss tgw routes`)

Finds which Transit Gateway route tables have a route to a CIDR, and which attachment each route
points to: a VPC, a VPN, a Direct Connect gateway, a peering (such as Cloud WAN) or a Connect
attachment, or nowhere (blackhole). It is the next step after
[`awss cloudwan routes`](#cloud-wan-routes-awss-cloudwan-routes) for the part of a network behind a
Transit Gateway: Cloud WAN routes the CIDR to a Transit Gateway route table, and this command shows
the VPC that holds it.

| Flag | Short | Description |
| --- | --- | --- |
| `--cidrs` | `-c` | CIDRs or IP addresses to find; a bare IP is its `/32` (IPv6 too). Without it, every route of the selected route tables is listed |
| `--match` | `-m` | How a route must relate to each CIDR: `any` (default: `exact`, `subnet-of` and `supernet-of` together), `exact`, `longest` (the route traffic takes), `subnet-of` (routes inside the CIDR), `supernet-of` (routes covering it) |
| `--route-table-ids` | | Route tables to search, exact |
| `--route-table-names` | | Route tables to search, by Name tag (globs) |
| `--transit-gateway-ids` | | Search the route tables of these transit gateways only, exact |
| `--types` | `-T` | Route types: `propagated`, `static` |
| `--states` | | Route states, such as `active` or `blackhole`; default both |
| `--attachment-types` | | Attachment types: `vpc`, `vpn`, `direct-connect-gateway` (`dx` for short), `peering`, `connect`... |
| `--attachment-ids` | | Attachment IDs, exact |
| `--resource-ids` | | Resources of the attachments, such as VPC IDs, exact |

```bash
# Cloud WAN sends 10.20.1.5 to tgw-rtb-0123 at eu-central-1: which VPC holds it?
awss tgw routes --profile network --regions eu-central-1 --route-table-ids tgw-rtb-0123 \
  --cidrs 10.20.1.5 --match longest

# The route points to an attachment whose Associated RT is tgw-rtb-0456: is there a return route
# to the source 10.60.3.7?
awss tgw routes --profile network --regions eu-central-1 --route-table-ids tgw-rtb-0456 \
  --cidrs 10.60.3.7 --match longest

# Is the data centre prefix learned over Direct Connect in every route table?
awss tgw routes --profile network --regions all --cidrs 10.50.0.0/16 --attachment-types dx

# Every route to one VPC
awss tgw routes --profile network --resource-ids vpc-0123456789abcdef0

# The blackholes of every route table
awss tgw routes --profile network --states blackhole
```

One row per attachment of each route (a route with several attachments, ECMP, is several rows):
**Query** (the CIDR searched), **TGW ID**, **Route Table ID** / **Name** (its Name tag), **CIDR** /
**Prefix List** (the destination: a CIDR, or the prefix list of a prefix list route), **Type**,
**State**, **Resource Type** / **Resource ID** (such as the VPC), **Attachment ID** / **Name** (its
Name tag), **Associated RT** / **Name** (the route table the attachment is associated with: the
table that routes the traffic coming back from the attachment, where to look for the return route;
empty when the attachment is not associated) and **Owner ID** / **Name** (the account of the resource; see
[Account names](#account-names)). Each pair is one cell, the second value on its own line.

**Checking the return route.** Search both addresses at once: `--match longest` shows, for each
CIDR, the route traffic takes in every route table, side by side (the **Query** column tells them
apart).

```bash
# Forward (to 10.20.1.5) and return (to 10.60.3.7) in every route table
awss tgw routes --profile network --regions eu-central-1 --cidrs 10.60.3.7,10.20.1.5 --match longest

# Only the two tables that matter: the source's, and the Associated RT of the destination's attachment
awss tgw routes --profile network --regions eu-central-1 --cidrs 10.60.3.7,10.20.1.5 --match longest \
  --route-table-ids tgw-rtb-0123,tgw-rtb-0456
```

The Associated RT is only one hop, so read it with the topology in mind:

- **Inspection:** behind a firewall (inspection) attachment, the Associated RT is the inspection
  table, not the source's; follow each hop.
- **No association:** a Direct Connect or VPN attachment may have no Associated RT; its return
  traffic follows the routes the other side advertises.
- **Cloud WAN:** behind a Cloud WAN peering, the return path is in
  [`awss cloudwan routes`](#cloud-wan-routes-awss-cloudwan-routes).

- **Profile:** run it with a profile of the account that owns the transit gateways, usually the
  network account. An account a transit gateway is shared with does not see its route tables, so
  its result set is empty.
- **Regional:** a transit gateway is regional, so the command searches every profile and region,
  as `ec2` does. Use `--regions` to pick the region of the transit gateway.
- **Calls:** one `DescribeTransitGatewayRouteTables` call, then one `SearchTransitGatewayRoutes`
  call per route table, per CIDR and per match mode (`any` is three, `longest` is two: `exact` and
  `supernet-of`, of which awss keeps the most specific route), then one
  `DescribeTransitGatewayAttachments` call for the names and owners. Every filter except
  `--route-table-ids` and `--route-table-names` is applied by AWS. When AWS says a route table has more routes than it
  returned, the result set reports it: narrow the search with `--cidrs`.
- **JSON only:** `announcement_id` (the route table announcement of a route from a Cloud WAN
  peering).
- **Permission:** `ec2:DescribeTransitGatewayRouteTables`, `ec2:SearchTransitGatewayRoutes`, and
  `ec2:DescribeTransitGatewayAttachments` for the attachment names and owners.

Sort by: `--sort route-table|route-table-id|tgw-id|cidr|prefix-list|query|type|state|resource-type|resource-id|attachment|attachment-id|associated-route-table|associated-route-table-id|owner|owner-name` (default: `route-table`). `cidr` sorts by address, and rows equal on the sort field stay in route table order: route table, then CIDR.

#### Transit Gateway attachments (`awss tgw attachments`)

Lists the Transit Gateway attachments: which VPC, VPN, Direct Connect gateway, peering (such as
Cloud WAN) or Connect attachment is attached to which transit gateway, owned by which account, and
the route table it is associated with: the table that routes the traffic coming from it.

| Flag | Short | Description |
| --- | --- | --- |
| `--ids` | `-i` | Attachment IDs, exact |
| `--names` | `-n` | Name tags (globs) |
| `--types` | `-T` | Resource types: `vpc`, `vpn`, `direct-connect-gateway` (`dx` for short), `peering`, `connect`... |
| `--resource-ids` | | Attached resources, such as VPC IDs, exact |
| `--states` | | States, such as `available`, `pending-acceptance`, `failed` |
| `--transit-gateway-ids` | | List the attachments of these transit gateways only, exact |
| `--associated-route-table-ids` | | Attachments associated with these route tables, exact |
| `--propagations` | | Add **Propagated RTs**: the route tables each attachment propagates to |
| `--propagated-route-table-ids` | | Attachments that propagate to these route tables, in any propagation state, exact; implies `--propagations` |

```bash
# The attachment of a VPC, and the route table it is associated with
awss tgw attachments --profile network --regions eu-central-1 --resource-ids vpc-0123456789abcdef0

# Everything associated with one route table
awss tgw attachments --profile network --regions eu-central-1 --associated-route-table-ids tgw-rtb-0123

# Peerings waiting for acceptance
awss tgw attachments --profile network --types peering --states pending-acceptance

# Why does a route table have, or lack, a propagated route to this VPC?
awss tgw attachments --profile network --regions eu-central-1 --resource-ids vpc-0123456789abcdef0 --propagations

# Everything that propagates to one route table
awss tgw attachments --profile network --regions eu-central-1 --propagated-route-table-ids tgw-rtb-0123
```

Columns: **Attachment ID** / **Name** (its Name tag), **TGW ID**, **Type**, **Resource ID** (such as
the VPC), **State**, **Associated RT** / **Name** (the associated route table and its Name tag;
empty when the attachment is not associated), **Propagated RTs** with `--propagations` (one route
table per line, `<id> (<name>)`, the state added when it is not `enabled`, such as
`tgw-rtb-0123 (dev, disabled)`; the column is hidden when no attachment propagates), **Owner ID** /
**Name** (the account of the resource; see [Account names](#account-names)), and **Tags** with
`--show-tags`. Each pair is one
cell, the second value on its own line.

- **Profile and region:** as for `tgw routes`, run it with a profile of the account that owns the
  transit gateways, in their region. Unlike `tgw routes`, an account a transit gateway is shared
  with is not empty: it sees the attachments of its own resources and their Associated RT, without
  the route table name, since it does not see the route tables.
- **Association state:** Associated RT is shown in any association state; the JSON
  `association_state` says `associated`, or `associating` / `disassociating` while it changes.
  `tgw routes` shows it only once associated.
- **Calls:** one `DescribeTransitGatewayAttachments` call, then one
  `DescribeTransitGatewayRouteTables` call for the route table names when an attachment is
  associated. `--names`, `--states` and `--propagated-route-table-ids` are matched by awss, the
  other filters by AWS. `--propagations` adds the propagation calls: AWS reads them one attachment
  or one route table at a time, so awss makes one `GetTransitGatewayAttachmentPropagations` call
  per attachment when there are fewer attachments than route tables (a single VPC is one call),
  otherwise one `GetTransitGatewayRouteTablePropagations` call per route table. A failed call is
  reported and the rows are kept.
- **JSON only:** `association_state`, `transit_gateway_owner_id`, `created`, and with
  `--propagations` the `propagations` (`route_table_id`, `route_table_name`, `state`).
- **Permission:** `ec2:DescribeTransitGatewayAttachments`,
  `ec2:DescribeTransitGatewayRouteTables` for the route table names, and with `--propagations`
  `ec2:GetTransitGatewayAttachmentPropagations` and `ec2:GetTransitGatewayRouteTablePropagations`.

Sort by: `--sort name|id|tgw-id|type|resource-id|state|associated-route-table|associated-route-table-id|owner|owner-name|created` (default: `name`).

### Cleanup

The cleanup commands are the only ones that change your AWS accounts. Each one shows its plan
and changes nothing unless `--apply` is given.

#### Default VPCs (`awss cleanup default-vpc`)

Deletes the default VPC of each profile and region, with the network plumbing inside it. The
rule: what exists only because of the VPC is deleted; anything that runs or holds data blocks
the deletion; for what lives outside the VPC, only its link to the VPC is removed.

| Flag | Description |
| --- | --- |
| `--apply` | Delete what the plan lists. Without it, nothing changes |
| `--confirm-all` | Required with `--apply` when the profiles or the regions are `all` (`--profiles all`, `--regions all`, or `profiles: all` / `regions: all` in the config file); refused otherwise |

The two flags are command-line only: the config file cannot turn on deleting.

```bash
# The plan for one account, every region: what would be deleted, and what blocks
awss cleanup default-vpc --profiles sandbox --regions all

# Delete in one region
awss cleanup default-vpc --profiles sandbox --regions eu-west-1 --apply

# Delete in every region of one account: --regions all needs --confirm-all
awss cleanup default-vpc --profiles sandbox --regions all --apply --confirm-all --timeout 30m

# Every profile and region; deleting waits for NAT gateways and endpoints, so raise --timeout
awss cleanup default-vpc --profiles all --regions all --apply --confirm-all --timeout 30m

# The blocked VPCs only, as JSON
awss cleanup default-vpc --profiles all --regions all --output json | jq 'select(.data[0].status == "blocked")'
```

**All or nothing.** Every service that runs in a VPC places a network interface (ENI) in it:
EC2, RDS, Lambda, load balancers, EFS, ECS, ElastiCache, MSK, OpenSearch, API Gateway VPC
links... One listing of the VPC's ENIs finds them all. If any ENI is not plumbing, the VPC is
`blocked` and nothing in it is touched, not even its endpoints or security groups: a partial
cleanup could break the workload.

| What | With `--apply` |
| --- | --- |
| VPC endpoints | deleted |
| NAT gateways | deleted, then their Elastic IPs are **released (permanent)**; only the IPs of those gateways. An IP AWS manages, as for a NAT gateway created without an Elastic IP, is not released: AWS frees it with the gateway, and the plan says `managed by AWS` |
| Peering connections | deleted; a request the VPC was asked to accept is rejected. Routes to it on the other side stop working |
| Transit Gateway attachment | deleted; **the Transit Gateway stays** |
| VPN gateway | **detached only**; the gateway, its VPN connections and the customer gateway stay |
| Detached ENIs of the account | deleted |
| Security groups but the default one | rules that reference them are revoked first (groups that reference each other), then deleted |
| Subnets, route tables but the main one, network ACLs but the default one | deleted |
| Internet gateway, egress-only internet gateways | detached and deleted |
| Flow logs | deleted; the logs they stored stay |
| The VPC | deleted last |
| An ENI in use, other than the plumbing above | **blocks** |
| A detached ENI managed by AWS (Lambda leftovers) | **blocks**: AWS removes it by itself, retry later |
| A VPC owned by another account | **blocks**: never touched |

Each profile and region with a default VPC prints one row: VPC ID, CIDR, status and details. A
region without a default VPC prints nothing (`--show-empty` shows it).

| Status | Meaning |
| --- | --- |
| `deletable` | Preview, no blocker: the details list what `--apply` deletes |
| `blocked` | The details name every blocker, such as `EC2 instance i-0abc (eni-1)` or `RDSNetworkInterface (eni-2)` |
| `denied` | Preview: a DryRun of `ec2:DeleteVpc` says this role may not delete VPCs |
| `deleted` | `--apply` deleted the VPC; the details list what was deleted |
| `failed` | `--apply` stopped at a step; the details give the step and the AWS error |

A run that stops partway (an error, the timeout, an instance launched during the run) leaves
what it deleted deleted. Run it again: it starts from what is left, and a resource already gone
counts as done. That includes the Elastic IP of a NAT gateway a previous run deleted: AWS lists a
deleted NAT gateway for about an hour, and the IP is released if it still exists, is managed by
the account and is associated with nothing else. A release that AWS refuses because the IP is still held by the
deleted gateway is tried again. A deleted default VPC can be recreated with
`aws ec2 create-default-vpc`.

A `failed` row shows the step and the AWS error (`Code: message`) on two lines. For the full
error, with the request ID, use `--output json`.

#### Unused DHCP options sets (`awss cleanup dhcp-options`)

Deletes the DHCP options sets of each profile and region that no VPC uses. A set is only used by
being associated with a VPC, so deleting an unused one changes nothing for anything running.
Deleting a VPC leaves its set behind, so run this after `cleanup default-vpc`.

It takes the same `--apply` and `--confirm-all` flags as `default-vpc`.

```bash
# The unused sets of one account, every region
awss cleanup dhcp-options --profiles sandbox --regions all

# After cleanup default-vpc: delete them in every region
awss cleanup dhcp-options --profiles sandbox --regions all --apply --confirm-all
```

- **Only the account's sets:** a participant of a shared VPC sees that VPC, but the VPC uses its
  owner's set, so only the sets the account owns are listed.
- **One row per unused set:** ID, Name tag, options, status and details. The options column shows
  the set's configuration, so a set someone created on purpose stands out from the AWS default
  (`domain-name: <region>.compute.internal`, `domain-name-servers: AmazonProvidedDNS`). `--show-tags`
  adds its tags. JSON has `options` as key → values. A region with no unused set prints nothing
  (`--show-empty` shows it).
- **Statuses:** `deletable`, `denied` (a DryRun of `ec2:DeleteDhcpOptions` says the role may
  not delete it), `deleted`, `failed` (the AWS error as `Code: message`, for example
  `DependencyViolation` if a VPC started using the set during the run). A set already gone counts
  as deleted.
- **Permissions:** the preview needs `ec2:DescribeDhcpOptions`, `ec2:DescribeVpcs` and
  `sts:GetCallerIdentity`; `--apply` also needs `ec2:DeleteDhcpOptions`.

The preview needs only `ec2:Describe*` and `sts:GetCallerIdentity`. `--apply` also needs:
`ec2:DeleteVpcEndpoints`, `ec2:DeleteNatGateway`, `ec2:ReleaseAddress`,
`ec2:DeleteVpcPeeringConnection`, `ec2:RejectVpcPeeringConnection`,
`ec2:DeleteTransitGatewayVpcAttachment`, `ec2:DetachVpnGateway`, `ec2:DeleteNetworkInterface`,
`ec2:RevokeSecurityGroupIngress`, `ec2:RevokeSecurityGroupEgress`, `ec2:DeleteSecurityGroup`,
`ec2:DeleteSubnet`, `ec2:DisassociateRouteTable`, `ec2:DeleteRouteTable`,
`ec2:DeleteNetworkAcl`, `ec2:DetachInternetGateway`, `ec2:DeleteInternetGateway`,
`ec2:DeleteEgressOnlyInternetGateway`, `ec2:DeleteFlowLogs` and `ec2:DeleteVpc`.

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

An ID that is not in a profile or region is not an error. When `ec2`, `eni`, `ebs`, `vpc` or
`subnet` search by ID (`--ids`), awss drops the IDs AWS reports as not found
(`InvalidInstanceID.NotFound`, ...) and asks again for the rest, so a set where none of them
exist is counted as empty.

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

`vpc`, `subnet`, `eni`, `cloudwan routes`, `cloudwan bgp`, `cloudwan attachments`, `dx gateways`, `tgw routes` and `tgw attachments` show the owner name (JSON `owner_name`, sort field `owner-name`)
next to **Owner ID**, in its own **Owner** column or on the line under the ID: the name of the account, which matters in a shared VPC, where the owner is
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
config file, makes the commands that show the owner name call `organizations:ListAccounts` once per run with
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

Each release has an `awss-<version>-checksums.txt` with the SHA-256 of every archive. Check the
archive you downloaded before you extract it:

```bash
VERSION=v0.17.0   # the release you downloaded

# Linux
sha256sum -c --ignore-missing "awss-${VERSION}-checksums.txt"

# macOS (Apple silicon; use darwin-amd64 on Intel)
grep "awss-${VERSION}-darwin-arm64.tar.gz" "awss-${VERSION}-checksums.txt" | shasum -a 256 -c -
```

Both print `OK` for a good archive. Releases up to v0.16.0 have a `.md5` file per archive
instead, holding only the hash.

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
- Read-only IAM permissions. The search commands never modify anything; only
  `awss cleanup ... --apply` deletes, and needs the permissions listed in [Cleanup](#cleanup). The
  minimum policy of the search commands is:

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

AWSS uses a YAML configuration file to set defaults. The default path is `~/.awss/config.yaml`, overridable with `--config`
or the `AWSS_CONFIG` environment variable (see [Several config files](#several-config-files)).

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

### Several config files

When the accounts are split, for example by permission level (admin, regular user) and by
landing zone (legacy, LZ2), keep one config file per combination in `~/.awss/`. Each has its own
`all-profiles`, `accounts` and defaults, so `--profiles all` stays inside one landing zone and one
permission level.

```text
~/.awss/config.yaml             # the default
~/.awss/devops-lz2.yaml
~/.awss/devops-legacy.yaml
~/.awss/godpowers-lz2.yaml
```

Pick one by **name**: a `--config` value with no `/` and no `.yaml` or `.yml` extension is read as
`~/.awss/<name>.yaml`. Any other value is a path, as before.

```bash
awss --config devops-lz2 vpc --all --profiles all     # ~/.awss/devops-lz2.yaml
awss --config ./team.yaml vpc --all                   # a path

# Every awss command of this shell session uses devops-lz2; --config still wins
export AWSS_CONFIG=devops-lz2
awss vpc --all --profiles all
```

- `AWSS_CONFIG` takes a name or a path too. A leading `~/` is the home directory, also inside the
  variable.
- A config asked for by name or path must exist: a missing name fails and lists the configs
  available, so a mistyped `AWSS_CONFIG` never falls back to the default silently. Only the
  default `~/.awss/config.yaml` may be missing.
- `--config <TAB>` completes the names in `~/.awss/`, and YAML file names once the value looks
  like a path (it has a `/`, or starts with `.` or `~`).

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

# Is a data centre prefix learned over Direct Connect in the Cloud WAN segments?
awss cloudwan routes --profile network --cidrs 10.50.0.0/16 --target-types direct-connect-gateway

# Which VPC behind the Transit Gateway holds 10.20.1.5?
awss tgw routes --profile network --regions eu-central-1 --cidrs 10.20.1.5 --match longest

# The plan to delete the default VPCs of one account, in every region (nothing changes)
awss cleanup default-vpc --profiles sandbox --regions all

# JSON output for scripting
awss ec2 --all --output json

# Every profile and region, with the run stats on stderr and the JSON results in a file
awss ec2 --all --profiles all --regions all --output json --stats > instances.json
```

### Shell completion

`awss completion zsh|bash|fish|powershell` prints a completion script. Besides commands and flag
names, it completes the values of these flags, one item after another in a comma list:

| Flag | Values |
| --- | --- |
| `--sort` | the sort fields of that command |
| `--output` | `json`, `json-pretty`, `table` |
| `--profiles`, `--org-profile` | the profiles of the AWS config file, and `all` for `--profiles` |
| `--regions` | the `all-regions` list of the config file (or its default), and `all` |
| `--config` | the config names in `~/.awss/`, or YAML file names for a path |
| `-z` / `--availability-zones` | the zone letters `a` to `f` |
| `ec2 --instance-states` | the EC2 instance states, such as `running`, `stopped` |
| `ebs --statuses`, `ebs --volume-types` | the EBS volume states and types |
| `vpc --states`, `subnet --states` | the VPC and subnet states |
| `ebs --encrypted`, `vpc --default`, `subnet --default-for-az`, `subnet --public-ip-on-launch` | `true`, `false` |
| `org --statuses` | the account statuses |

The other flags take free text (IDs, names, tags, CIDRs, numbers), and `<TAB>` after them offers
nothing rather than file names. Completion never calls AWS.

```bash
# zsh: load it in every new shell
echo 'source <(awss completion zsh)' >> ~/.zshrc

# bash (needs the bash-completion package)
echo 'source <(awss completion bash)' >> ~/.bashrc
```

Then `awss rds --sort <TAB>` lists `az class cluster engine id status version vpc`.

The search commands take no arguments, only flags, and the shell lists flag names once a `-` is
typed. So `awss ec2 <TAB>` offers no file names. In bash and zsh it shows a hint instead:
`ec2 takes no arguments; type - and press TAB to see its flags`. Set `AWSS_ACTIVE_HELP=0` to
hide the hint, or `COBRA_ACTIVE_HELP=0` to hide it for every Cobra-based tool.

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
