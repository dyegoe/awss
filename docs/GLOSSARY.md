# Glossary

The words used in the awss code, the README and the issues, in plain language. Each term has
one meaning here. When the code or a doc uses a term in another sense, rename it or fix the
definition, so it doesn't end up with two meanings.

A pull request that introduces a term adds it here (`AGENTS.md`, Standards and guardrails). The
terms are in alphabetical order within each section.

- [AWS terms](#aws-terms): the AWS services and networking words awss searches.
- [awss terms](#awss-terms): what a user sees when running awss.
- [Code terms](#code-terms): the names inside the code.

## AWS terms

**Account.** An AWS account, named by a 12-digit ID. Every resource belongs to one account, its
owner. A shared VPC or a Cloud WAN attachment can be owned by another account than the one
searching.

**Allowed prefix.** A CIDR that a Direct Connect gateway advertises to the data centres through
one association to a transit gateway or a virtual private gateway. An association to a Cloud WAN
core network has none: Cloud WAN advertises the routes of the segment instead.

**AS number (ASN), AS path.** A BGP network is named by an AS number, such as `65010`. The AS path
of a BGP route lists the AS numbers the route went through, the origin last.

**Association (Transit Gateway).** The link from an attachment to the one route table that routes
the traffic coming from it. awss shows it as the **Associated RT**. Compare *propagation*.

**Association (Direct Connect).** The link from a Direct Connect gateway to a transit gateway, a
virtual private gateway or a Cloud WAN core network, with its allowed prefixes.

**Attachment.** What connects a network to a Cloud WAN core network or a transit gateway: a VPC,
a VPN, a Direct Connect gateway, a Connect attachment, a peering, or (Cloud WAN) a Transit Gateway
route table. A route points to an attachment as its next hop.

**BGP.** The routing protocol that networks use to tell each other which prefixes they can reach.
A BGP route carries the prefix, its AS path, communities, local preference, MED and next hop.

**Blackhole.** A route that drops the traffic: it points nowhere.

**CIDR.** A block of IP addresses, such as `10.20.0.0/16`. A bare IP is its own `/32` (`/128`
for IPv6).

**Cloud WAN.** The AWS service that builds one global network (a *core network*) from segments
and edge locations. Its API is Network Manager.

**Community (BGP).** A tag on a BGP route, such as `65000:100`, used by routing policies to decide
what to do with it.

**Connect attachment, Connect peer.** An attachment that runs BGP over a tunnel, usually to an
SD-WAN appliance; the Connect peer is one BGP session of it.

**Core network.** One Cloud WAN network: its segments, edge locations, attachments and routing
policies. It is owned by one account, usually the network account, and can be shared.

**Default VPC.** The VPC AWS creates in every region of a new account. `awss cleanup default-vpc`
deletes it when no workload uses it.

**DHCP options set.** The DNS settings a VPC hands to its instances. A set no VPC uses is unused;
`awss cleanup dhcp-options` deletes those.

**Direct Connect (DX).** The AWS service for private links to the data centres. A *Direct Connect
gateway* is global and joins the virtual interfaces (VIFs) to the AWS side.

**ECMP.** Equal-cost multi-path: one route with several next hops, traffic spread over them.

**Edge location (edge).** A region where a Cloud WAN core network is present. Each segment has a
route table at each edge.

**Installed (awss cloudwan bgp).** Whether a BGP route made it into the route table: `yes` (the
installed route points to its attachment), `no` (it points elsewhere), `none` (no route of the
prefix is installed). See the README, Cloud WAN BGP routes.

**Match mode.** How a route must relate to a searched CIDR: `exact`, `subnet-of` (inside it),
`supernet-of` (covering it), `longest` (the most specific covering route: the one traffic
takes), or `any` (exact, subnet-of and supernet-of together).

**Network account.** The account that owns the shared network: the Cloud WAN core network, the
transit gateways, the Direct Connect gateways and the shared subnets. Run the networking commands
with one of its profiles.

**Network function group (NFG).** A Cloud WAN group of inspection attachments, with route tables
like a segment's. awss writes it `nfg:<name>` in the Segment column.

**Next hop.** Where a route sends the traffic: an attachment, or nowhere (blackhole).

**Organization (AWS Organizations).** The group of accounts under one management account. awss
reads it with `awss org`, and for account names with `--org-profile`.

**Peering.** An attachment between a transit gateway and another transit gateway or a Cloud WAN
core network.

**Prefix.** A CIDR as routing sees it: the destination of a route.

**Prefix list.** A named set of CIDRs that one route can point to instead of a single CIDR.

**Profile.** A named set of AWS credentials in the AWS config file, usually one role in one
account. One account can have several profiles.

**Propagation.** An attachment telling a route table about its routes, so the table learns them
as *propagated* routes. One attachment can propagate to many route tables. Compare *association*.

**Region.** An AWS region, such as `eu-west-1`. Global services (Organizations, CloudFront,
Network Manager, Direct Connect gateways) answer once per profile, not per region.

**Route table.** A list of routes: for each prefix, its next hop. Cloud WAN has one per segment
(or network function group) and edge; a transit gateway has as many as it is given.

**Route type.** How a route got into a table: `propagated` (learned from an attachment), `static`
(written by hand), `system` (made by Cloud WAN).

**Routing policy.** Cloud WAN rules that drop, change or summarise BGP routes before they are
installed. `awss cloudwan bgp` shows the routes before the policies, `awss cloudwan routes` after.

**Segment.** A Cloud WAN routing domain, such as `prod` or `dev`: the attachments of one segment
reach each other, and other segments only when the policy shares routes.

**Shared VPC.** A VPC whose subnets are owned by the network account and shared with other
accounts, which launch their resources in them. The owner of the subnet is not the owner of
the instance.

**Transit gateway (TGW).** A regional router that joins VPCs, VPNs, Direct Connect gateways and
peerings, with its own route tables.

**VIF (virtual interface).** One BGP session over a Direct Connect link, attached to a Direct
Connect gateway.

**Virtual private gateway (VGW).** The VPN or Direct Connect endpoint of one VPC.

## awss terms

**Account names.** The names shown in the **Owner** column next to an account ID, from the
`accounts` map of the config file or, with `--org-profile`, from AWS Organizations.

**Agent harness.** A tool that runs an LLM with access to a shell, such as Claude Code, GitHub
Copilot or Antigravity. It loads agent skills.

**Agent skill.** A `SKILL.md` file (Agent Skills format: YAML front matter with `name` and
`description`, then Markdown) that a harness loads when its description fits the task.
`awss skill` prints the one for awss.

**Apply, plan.** A cleanup command only reports its plan (what it would delete) unless `--apply`
is given. Over `--profiles all` or `--regions all`, `--apply` also needs `--confirm-all`.

**AWS filter.** A filter sent to AWS in the call, so AWS returns only the matching resources.

**Client-side filter.** A filter awss matches on the results, because the AWS call cannot: names
with globs, versions, tags of some services, the Cloud WAN and Direct Connect filters. Compare
*AWS filter*.

**Fields section.** The end of every command's `--help`: one line per field of the rows, with
its JSON name, its table column (or "JSON only"), its sort field and the first sentence of its doc
comment.

**Fixed-list flag.** A flag that accepts only a known list of values, such as `--states`. Its help
lists the values, and shell completion offers them.

**Global.** A search that runs once per profile instead of once per region. Its result set shows
the region `global`.

**Glob.** A name pattern with `*` (any text) and `?` (one character), such as `web-*`. With
`--regex`, the patterns are regular expressions instead.

**Hidden column.** A table column left out when it is empty in every row, such as one that only a
flag fills.

**Query.** The column that names which searched CIDR a row answers, when one run searches several.

**Result set.** The rows of one profile and region, with its errors. The table prints one per
profile and region; JSON prints one object per profile and region.

**Run.** One awss command: every profile × region search it makes, at most `--concurrency` at
once, bounded by `--timeout`.

**Search.** The work for one profile and one region: AWS calls, then rows. A failing search
records its error in its result set; the other searches go on.

**Sort field.** A value of `--sort`, such as `id` or `cidr`. Each command lists its own.

**Stacked column.** A table cell holding two or three values on separate lines, such as an ID and
its name under it. JSON keeps one field for each.

**Timed out.** A search still running, or still waiting to start, at the `--timeout` deadline. Its
result set holds only the timeout error: rows cut short could be incomplete.

## Code terms

**`BaseResults`.** The fields every result set has: profile, region, errors and sort field, with
their getters and `AddError` (`common/results.go`). Embedded in `RowSet`.

**`cmdSpec`.** What a command's `RunE` tells `runSearch`: the viper keys of its flags and which
optional flags it has (`cmd/root.go`).

**Engine.** A `search.Engine`: how a command builds its result sets (`New`), which sort fields it
accepts, and whether it shows account names. Each command file declares its own, such as
`vpcEngine`.

**Fan-out.** `search.Execute` running one search per profile and region on a pool of workers.

**`fieldDocs`.** The generated map (`cmd/fielddocs_gen.go`) of each row field's description, written
by `go generate ./cmd` (`internal/fielddoc`) from the doc comments.

**Order function.** A function such as `orderRows` that sorts the rows of an engine its own way,
passed to `RowSet.SortRowsBy` (eni, dxgw, tgw, cloudwan).

**`Results`.** The interface every result set implements (`common.Results`): `Search(ctx)`, the
rows and headers, the profile, region and errors. The runner and the output see only this.

**Row type.** The struct of one table row, usually `dataRow`. Its struct tags drive the output:
`json`, `header`, `sort`, `stack` (stacked column) and `hide:"empty"` (hidden column).

**`RowSet`.** The generic base a result type embeds (`common.RowSet[dataRow]`): it holds the rows
and gives `Len`, `GetHeaders`, `GetRows` and the sort step (`SortRows`, `SortRowsBy`).

**Subcommand.** The `subcommand` value a command file declares (`cmd/root.go` type): the cobra
command, its engine, its fixed-list flag values and the functions that register and bind its
flags. The `subcommands` list registers them.

**Viper key, label.** The name a flag is bound to in viper and the config file, such as
`ec2.sort`. The constants that hold them are named `label…`.
