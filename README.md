# AWSS

AWSS (stands for AWS Search) is a CLI tool to make your life easier when searching AWS resources.

It is a wrapper written in Go using AWS SDK Go v2. The work is still in progress and will be updated regularly.

## Version

<!-- Do not forget to update version on commands/commands.go Version -->
The current version is 0.6.0

## Features

- Specify a configuration file `--config`
- Search AWS resources
  - in parallel (default)
  - using all profiles, multiple profiles or a single profile. `--profiles all` or `--profiles default,dev`
  - in all regions, multiple regions or a single region. `--regions all` or `--regions us-east-1,us-east-2`
- Select the output format
  - table `--output table` (default)
  - json `--output json`
  - json-pretty `--output json-pretty`
- Show empty results `--show-empty`
- Show tags `--show-tags` (it works only with `--output table` and it is ignored when using another output format)
- Search AWS ec2 instances `awss ec2`
  - Filter by:
    - instance ids `--instance-ids|-i i-1234567890abcdef0,i-0987654321fedcba0`
    - names `--names|-n my-instance-1,my-instance-2`
    - tags `--tags|-t Name:my-instance-1,Name:my-instance-2`
    - tag keys `--tag-keys|-k Name,Environment`
    - instance types `--instance-types|-T t2.micro,t3.micro`
    - availability zones `--availability-zones|-z a,b`
    - instance states `--instance-states|-s running,stopped`
    - private ips `--private-ips|-p 172.16.0.1,172.17.1.254`
    - public ips `--public-ips|-P 52.28.19.20,52.30.31.32`
  - Sort by:
    - id `--sort id`
    - name `--sort name`
    - type `--sort type`
    - az `--sort az`
    - state `--sort state`
    - private ip `--sort private_ip`
    - public ip `--sort public_ip`
    - enis `--sort enis`

And you can combine these filters together.

## Installation

```txt
git clone http://github.com/dyegoe/awss
cd awss
go build
cp awss /usr/local/bin
```

Or you can download the binary from the [releases](https://github.com/dyegoe/awss/releases) page.

## Configuration

AWSS uses a configuration file to set the default:

- profiles
- regions
- output format
- show empty results
- show tags
- all regions to search
- sort field for ec2 instances

You can create a configuration file on your home directory `~/.awss/config.yaml` or use the `--config` flag to specify a config file.

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
```

## Usage

```txt
awss --help
awss ec2 --help
awss --profiles default,profile \
  --regions eu-central-1,us-east-1,sa-east-1 ec2 \
  --names 'some-wildcard*' \
  --tags Environment:dev,Environment:prod \
  --tag-keys 'zone-*' \
  --instance-types t2.micro,t3.medium \
  --availability-zones a,b \
  --instance-states running,stopped \
  --private-ips 172.16.0.1,172.17.1.254 \
  --public-ips 52.28.19.20,52.30.31.32 \
  --sort id \
  --output table \
  --show-empty \
  --show-tags
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
- [Go-Pretty](https://github.com/jedib0t/go-pretty)
- [Go release binaries](https://github.com/marketplace/actions/go-release-binaries)
