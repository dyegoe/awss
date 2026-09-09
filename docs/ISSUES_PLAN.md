# Issues Plan — awss

Implementation plan for every open GitHub issue as of 2026-09-09.
Same conventions as `docs/FIXPLAN.md`: work top-to-bottom, one issue-numbered branch + PR per step,
run `go build ./... && go test ./... && golangci-lint run` before each PR, check off items here.

Legend: [ ] to do · [x] done · [~] in progress

---

## Open issues and order

| Order | Issue | Title                                     | Size | Depends on |
| ----- | ----- | ----------------------------------------- | ---- | ---------- |
| 1     | #72   | Add volumes to `search/ec2`               | S    | —          |
| 2     | #78   | Initial support for VPC search            | M    | (step 0)   |
| 3     | #79   | Initial support for Subnet search         | M    | (step 0)   |
| 4     | #82   | Search EC2 instances by CIDR              | M    | #78, #79   |
| 5     | #23   | Initial support for S3                    | L    | (step 0)   |

Why this order: #72 is a self-contained quick win inside an existing package. #78 and #79 are
copies of the `ebs` pattern and unblock #82, which is explicitly specified in terms of the
subnet and VPC lookups. #23 goes last: it brings a new SDK module, breaks the "one API call per
profile x region" assumption, and needs two design decisions from you (see step 5).

---

## Facts verified against the code and the pinned SDK (v1.296.1 for ec2)

- `ec2.sortResults` compares `reflect.Value.String()`. For a `[]string` field that returns the
  placeholder `"<[]string Value>"`, so the existing `sort:"enis"` tag never reorders rows
  (verified with a throwaway test). Any new slice column must not repeat this.
- `search.Execute` hands the **same** `filters` map to every `New()` call. Anything that
  rewrites filters per profile x region inside `Search()` must work on a copy.
- `StructToFilters` only handles `[]string` and `[]net.IP`. It emits every tagged, non-empty
  field, so a pseudo-filter such as `cidr` reaches `getFilters` and would fall into the
  `default` case and be sent to AWS as a real filter name.
- `runSearch` tolerates a nil `azs` slice (`buildFilters` swallows `ErrNoAZSelected`).
- DescribeInstances filters: `block-device-mapping.volume-id`, `subnet-id`, `vpc-id`.
- DescribeVpcs filters: `vpc-id`, `cidr` (primary, exact match), `cidr-block-association.cidr-block`,
  `state`, `is-default`, `owner-id`, `dhcp-options-id`, `tag`, `tag-key`. Paginator exists.
- DescribeSubnets filters: `subnet-id`, `vpc-id`, `cidr-block` (exact match), `availability-zone`,
  `state`, `default-for-az`, `map-public-ip-on-launch`, `owner-id`, `tag`, `tag-key`. Paginator exists.
- S3 SDK (`service/s3` v1.112.0): `ListBucketsInput` has `BucketRegion`, `Prefix`, `MaxBuckets`,
  `ContinuationToken`; `types.Bucket` has `Name`, `CreationDate`, `BucketRegion`, `BucketArn`;
  `NewListBucketsPaginator` and `NewListObjectsV2Paginator` exist. So a per-region goroutine can
  ask S3 for only that region's buckets and the existing fan-out holds.
- `pflag` v1.0.10 has `IPNetSliceVarP`, but see step 0b for why the plan uses `[]string` instead.
- `common.TagsToMap` / `common.TagName` take `ec2/types.Tag`. S3 uses a different tag type.

---

## Step 0 — Shared groundwork (decided 2026-09-09: do all of 0a, 0b, 0c before #78)

Not in any issue. Each item removes boilerplate that four new commands would otherwise copy.
Ordering: #72 ships first **without** a `sort` tag on the new slice column; 0c then lands
before #78 and retrofits `sort:"volumes"` (and fixes `enis`) in the same PR.

- [x] **0a: `search.Options` struct + constructor registry** (`search/search.go`)
  - `Execute` already carries a resource-specific `noInstanceName bool`. S3 adds `regex bool`
    and later `keys`. Replace the trailing args with `search.Options{SortField, NoInstanceName, Regex string...}`.
  - Replace the `switch cmd` with `var constructors = map[string]func(profile, region string, filters map[string][]string, opts Options) common.Results`.
  - `getSortFieldsCMDList` and `constructors` then become the single registration point.
  - Tests: `search_test.go` already mocks `getSortFieldsCMDList`; add "unknown command returns error".

- [x] **0b: `common.CheckCIDRs([]string) error`** (`common/aws.go` or `common/common.go`)
  - Validates each value with `net.ParseCIDR`. Used by #78, #79, #82 flag validation.
  - Decision: keep CIDR flags as `[]string` (not `[]net.IPNet`) so `StructToFilters` needs no new
    case and `*` wildcards still pass through to AWS filters.
  - Values containing `*` skip `net.ParseCIDR` (AWS CIDR filters accept wildcards); all other
    values must parse.
  - Tests: valid v4, invalid string, `10.0.*` accepted, `*` accepted.

- [x] **0c: Fix slice-field sorting** (`search/ec2/ec2.go`, then reuse)
  - Add `common.SortKey(v reflect.Value) string` that returns `String()` for strings, the
    decimal for ints, and `strings.Join(sorted slice, ",")` for `[]string`.
  - Use it in `ec2.sortResults` (fixes `enis`) and in every new package's `sortResults`.
  - Optionally lift `GetHeaders` / `GetRows` / `GetSortFields` into generic helpers in `common`
    (`common.HeadersOf(dataRow{})`, `common.SortFieldsOf(dataRow{}, f)`). Each new package
    currently copies ~80 identical lines.
  - Tests: sort by `enis` actually reorders; numeric sort still works for ebs `size`.

---

## Step 1 — #72: Add volumes to `search/ec2`

Branch: `72-add-volumes-to-ec2`

- [x] **Show attached volumes.** In `search/ec2/ec2.go` add to `dataRow`:
  `Volumes []string \`json:"volumes,omitempty" header:"Volumes"\``.
  Populate in `parseInstance` from `inst.BlockDeviceMappings[].Ebs.VolumeId`.
  Nil-check `Ebs` and `VolumeId`. No extra API call needed.
  Ship without the `sort` tag (slice sorting is a no-op today); 0c adds it back.
- [x] **Search by volume id.** In `cmd/ec2.go`:
  - `VolumeIDs []string \`filter:"block-device-mapping.volume-id"\`` on `ec2Filters`.
  - Flag `--volume-ids` / `-v` (short letter is free; ec2 uses a i n t k T z s p P). Cobra's
    `--version`/`-v` lives on the root command only, but confirm with `awss ec2 --help` that
    `-v` binds to `--volume-ids`; fall back to `-V` if it clashes.
  - Append `"volume-ids"` to `ec2FilterFlags` so `--all` exclusivity holds.
  - Update the command `Long` help text list of filters.
- [x] **Docs.** README ec2 filter table, sort list, usage example `awss ec2 --volume-ids vol-...`.
- [x] **Tests** (`search/ec2/ec2_test.go`, `cmd/`):
  - `parseInstance`: mapping with nil `Ebs`, nil `VolumeId`, two volumes, no mappings.
  - `getFilters`: `block-device-mapping.volume-id` goes through the default case as a `types.Filter`.
  - `GetHeaders` includes `Volumes`; `GetSortFields` includes `volumes` (if tagged).

---

## Step 2 — #78: VPC search (`awss vpc`)

Branch: `78-vpc-search`. Copy the `ebs` package shape (paginator, helpers under 60 lines).

- [x] **`search/vpc/vpc.go`**
  - `Results{common.BaseResults; Data []dataRow; Filters map[string][]string}`.
  - `dataRow` (all with `json`/`header`, sortable ones with `sort`):
    `VpcID` (id), `Name` (name, from tag:Name), `CidrBlock` (cidr), `CidrBlocks []string`
    (all associations, header only unless 0c), `State` (state), `IsDefault string` (default,
    `strconv.FormatBool`), `OwnerID` (owner), `DhcpOptionsID` (dhcp), `Tags map[string]string`.
  - `getFilters`: `vpc-id` -> `input.VpcIds`; `tag:Name` -> `common.FilterNames`;
    `tag` -> `common.FilterTags`; everything else -> `common.FilterDefault`.
  - `Search`: `NewDescribeVpcsPaginator`, `parseVpc(*types.Vpc) dataRow`, sort.
  - **Export `IDsByCIDR(ctx, profile, region string, cidrs []string) ([]string, error)`**: runs a
    search with filter `cidr-block-association.cidr-block` (covers primary and secondary blocks)
    and returns the VPC IDs. Needed by #82. Must not import `search/ec2` (cycle).
- [x] **`cmd/vpc.go`**
  - `vpcFilters`: `IDs "vpc-id"`, `Names "tag:Name"`, `Tags "tag"`, `TagsKey "tag-key"`,
    `CIDRs "cidr-block-association.cidr-block"`, `States "state"`, `IsDefault "is-default"`,
    `OwnerIDs "owner-id"`.
  - Flags: `-a --all`, `-i --ids`, `-n --names`, `-t --tags`, `-k --tags-key`, `-c --cidrs`,
    `-s --states`, `-d --default`, `-o --owner-ids`, `--sort` (default `name`).
  - `vpcRunE` validates `vpcF.CIDRs` with `common.CheckCIDRs`, then calls
    `runSearch(cmd, labelVpcAll, labelVpcSort, "", vpcFilterFlags, nil, vpcF.Tags, vpcF)`
    (VPCs have no AZ).
- [x] **Wire-up**: `vpcInitFlags()` / `vpcInitViper()` in `cmd/root.go`, `case "vpc"` in
  `search.Execute` (or registry from 0a), `"vpc": searchVPC.GetSortFields`.
- [x] **Docs**: README section + config example `vpc: sort: name`; CLAUDE.md package layout.
- [x] **Tests**: `New`, `getFilters` (ids path, tag path, malformed tag error, default path),
  `parseVpc` with nil `IsDefault`/`OwnerId`/empty association set, `sortResults`, `GetSortFields`
  invalid field message, `IDsByCIDR` with the search function injected via a package var.

---

## Step 3 — #79: Subnet search (`awss subnet`)

Branch: `79-subnet-search`. Same shape as step 2.

- [x] **`search/subnet/subnet.go`**
  - `dataRow`: `SubnetID` (id), `Name` (name), `VpcID` (vpc-id), `CidrBlock` (cidr),
    `AvailabilityZone` (az), `AvailableIPs int32` (available-ips, numeric compare like ebs `Size`),
    `State` (state), `MapPublicIP string` (public-ip), `DefaultForAz string` (default),
    `OwnerID` (owner), `Tags`.
  - `getFilters`: `subnet-id` -> `input.SubnetIds`; `tag:Name`; `tag`;
    `availability-zone` -> `common.FilterAvailabilityZones(values, r.Region)`; default.
  - `Search`: `NewDescribeSubnetsPaginator`, `parseSubnet`, sort.
  - **Export `IDsByCIDR(ctx, profile, region string, cidrs []string) ([]string, error)`** using
    filter `cidr-block`. Needed by #82.
- [x] **`cmd/subnet.go`**
  - `subnetFilters`: `IDs "subnet-id"`, `Names "tag:Name"`, `Tags`, `TagsKey`,
    `VpcIDs "vpc-id"`, `CIDRs "cidr-block"`, `AvailabilityZones "availability-zone"`,
    `States "state"`, `DefaultForAz "default-for-az"`, `MapPublicIP "map-public-ip-on-launch"`.
  - Flags: `-a -i -n -t -k`, `-V --vpc-ids`, `-c --cidrs`, `-z --availability-zones`,
    `-s --states`, `-d --default-for-az`, `-p --public-ip`, `--sort` (default `name`).
  - `subnetRunE`: `CheckCIDRs`, then `runSearch(..., subnetF.AvailabilityZones, subnetF.Tags, subnetF)`.
- [x] **Wire-up, docs, tests**: same checklist as step 2. Extra parse cases: nil
  `AvailableIpAddressCount`, nil `MapPublicIpOnLaunch`, nil `DefaultForAz`.

---

## Step 4 — #82: Search EC2 instances by CIDR

Branch: `82-ec2-search-by-cidr`. Semantics exactly as the issue: subnets first, then VPCs,
else error.

- [ ] **Flag** in `cmd/ec2.go`: `CIDRs []string \`filter:"cidr"\`` on `ec2Filters`, flag
  `--cidrs` / `-c`, validated with `common.CheckCIDRs`, added to `ec2FilterFlags`.
  Comment on the field: `cidr` is a pseudo-filter resolved inside `search/ec2`, not an AWS filter.
- [ ] **Resolution inside `search/ec2`** (it must run per profile x region because IDs differ per region):
  - Package vars for mocking: `var subnetIDsByCIDR = searchSubnet.IDsByCIDR` and
    `var vpcIDsByCIDR = searchVPC.IDsByCIDR`. `search/ec2` now imports `subnet` and `vpc`;
    those two must never import `ec2` (eni/ebs already import ec2).
  - New method `resolveCIDRFilter(ctx) (map[string][]string, error)` that **copies** `r.Filters`,
    removes `cidr`, and adds either `subnet-id: ids` or `vpc-id: ids`. Never mutate `r.Filters`
    (shared across goroutines).
  - Filter names: `subnet-id` / `vpc-id` match the instance's primary ENI. If instances with
    secondary ENIs in other subnets should also match, use `network-interface.subnet-id` /
    `network-interface.vpc-id` instead. Plan assumes primary-only (simpler, matches the issue).
  - Cascade: subnets found -> `subnet-id`; none -> VPCs found -> `vpc-id`; none -> append
    `"no subnet or VPC found for CIDR(s) X in <region>"` to `r.Errors` and return with empty
    data (CODESTYLE §1: per-region errors go into `r.Errors`, never abort the run).
  - `getFilters` takes the resolved map; add `case "cidr":` guard that errors if it ever reaches
    there unresolved, so it cannot fall into `default` and be sent to AWS.
- [ ] **Docs**: README ec2 table + example `awss ec2 --cidrs 10.0.1.0/24`; note that CIDR must
  exactly match a subnet or VPC block (AWS filter semantics), `*` works.
- [ ] **Tests**: mock the two lookups; cases: subnet hit, subnet miss + VPC hit, both miss ->
  error in `r.Errors` and no AWS call, lookup error propagates to `r.Errors`, original
  `r.Filters` untouched after resolution (`reflect.DeepEqual` before/after), `getFilters` rejects
  a raw `cidr` key.

Follow-up idea (not in the issue): a `--cidr-contains` mode that filters client-side on private
IP membership for CIDRs broader than one subnet. Leave out unless asked.

---

## Step 5 — #23: S3 (`awss s3`)

Branch: `23-s3-search`. Largest step; two decisions needed first.

**Decision A (decided: option 1) — key search output shape.** `GetHeaders` reflects over one `dataRow` per package,
so bucket rows and object rows cannot share a package cleanly. Options:

1. **Two commands** (recommended): `awss s3` lists buckets; `awss s3obj` (or `s3keys`) lists
   objects and requires `--buckets`. Clean fit for the architecture, two small packages.
2. One command with `--keys` switching to an object-row layout: needs two dataRow types and
   special-casing in the output layer, which CLAUDE.md forbids.
3. Ship buckets now, defer keys to a follow-up issue.

**Decision B (decided) — wildcard vs regex.** `--names` uses glob matching via `path.Match`
(`*`, `?`, consistent with the `*` wildcard elsewhere) and a `--regex` bool switches both
`--names` and `--keys` to Go `regexp`. Matching is client-side either way.

Decided 2026-09-09: A = two commands (`awss s3`, `awss s3obj`); B = glob by default plus `--regex`.

- [ ] **5a: dependency.** `go get github.com/aws/aws-sdk-go-v2/service/s3` (depguard already
  allows the `aws-sdk-go-v2/` prefix). Run `go mod tidy`.
- [ ] **5b: `search/s3/s3.go` (buckets)**
  - `dataRow`: `Name` (name), `Region` (region), `CreationDate string` (created, RFC3339 so
    string sort is chronological), `ARN`.
  - `Search`: `ListBucketsPaginator` with `BucketRegion: r.Region` and `Prefix` set to the
    literal prefix of the pattern (glob: text before the first `*`/`?`/`[`; regex:
    `regexp.Regexp.LiteralPrefix()`); only when exactly one pattern is given. Then
    `matchName(name)` client-side.
  - Filters map keys: `name` (patterns) only. `--all` returns every bucket in the region.
  - No tags in phase one (`GetBucketTagging` is N+1 and uses a different tag type). Note in README.
  - Pre-auth via `common.WhoAmI` already works (STS, not service-specific).
- [ ] **5c: `search/s3obj/s3obj.go` (objects)** — only if Decision A = 1
  - `dataRow`: `Bucket`, `Key` (key), `Size int64` (size, numeric compare), `LastModified`
    (modified, RFC3339), `StorageClass` (class), `ETag`.
  - Requires `--buckets` (exact names, comma list) and `--keys` (patterns). Uses
    `ListObjectsV2Paginator` per bucket with `Prefix` derived from the pattern.
  - Region check: skip a bucket whose `BucketRegion` (from a single `ListBuckets` call) is not
    `r.Region`, so `--regions all` does not list the same bucket 17 times.
  - Add `--max-keys` guard (default e.g. 10000) that appends a warning to `r.Errors` when hit.
- [ ] **5d: `common.MatchPattern(value string, patterns []string, regex bool) (bool, error)`**
  with compiled-regex caching; unit tested (glob, regex, invalid regex error).
- [ ] **5e: `cmd/s3.go`, `cmd/s3obj.go`**: flags `-a --all`, `-n --names`, `--regex`, `--sort`;
  s3obj adds `-b --buckets`, `-K --keys`, `--max-keys`. Both pass `nil` for AZs.
  `regex` travels through `search.Options` (0a) or, without 0a, as a filters-map key `regex: ["true"]`.
- [ ] **5f: wire-up, README, config example (`s3: sort: name`), CLAUDE.md layout.**
- [ ] **5g: tests**: `matchName` table (glob, regex, prefix derivation), `parseBucket` with nil
  `CreationDate`/`BucketRegion`, region skip logic, `--buckets` required error, sort numeric on size.

---

## Cross-cutting checklist (apply to every step)

- All nine items of CLAUDE.md "Adding new resource types".
- `README.md`: filter table, sort list, extra flags, config example block, usage example.
- `CLAUDE.md`: package layout list.
- Commit format: `feat(search/vpc): add VPC search (#78)` etc., one logical change per commit.
- Verify: `go build ./... && go test ./... && golangci-lint run`, plus `awss <cmd> --help` smoke test.
- Keep `Search()` and helpers under `funlen` (60 lines) by using the ebs split
  (`newClient` / `collectRows` / `appendRows` / `sortIfRequested`).
