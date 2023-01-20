# AWSS

AWSS (stands for AWS Search) is a CLI tool to make your life easier when searching AWS resources.

It is a wrapper written in Go using AWS SDK Go v2. The work is still in progress and will be updated regularly.

## Version

<!-- Do not forget to update version on cmd/root.go Version -->
The current version is 0.7.2

## Features

* Specify a configuration file `--config`
* Search AWS resources
  * in parallel (default)
  * using all profiles, multiple profiles or a single profile. `--profiles all` or `--profiles default,dev`
  * using all regions, multiple regions or a single region. `--regions all` or `--regions us-east-1,us-east-2`
* Select the output format
  * table `--output table` (default)
  * json `--output json`
  * json-pretty `--output json-pretty`
* Show empty results `--show-empty`
* Show tags `--show-tags` (it works only with `--output table` and it is ignored when using another output format)
* Search AWS ec2 instances `awss ec2`
  * Filter by:
    * instance ids `--instance-ids|-i i-1234567890abcdef0,i-0987654321fedcba0`
    * names `--names|-n my-instance-1,my-instance-2`
    * tags `--tags|-t Name:my-instance-1,Name:my-instance-2`
    * tag keys `--tag-keys|-k Name,Environment`
    * instance types `--instance-types|-T t2.micro,t3.micro`
    * availability zones `--availability-zones|-z a,b`
    * instance states `--instance-states|-s running,stopped`
    * private ips `--private-ips|-p 172.16.0.1,172.17.1.254`
    * public ips `--public-ips|-P 52.28.19.20,52.30.31.32`
  * Sort by:
    * id `--sort id`
    * name `--sort name`
    * type `--sort type`
    * az `--sort az`
    * state `--sort state`
    * private ip `--sort private_ip`
    * public ip `--sort public_ip`
    * enis `--sort enis`
* Search AWS ENIs
  * Filter by:
    * network interface ids `--network-interface-ids|-i eni-1234567890abcdef0,eni-0987654321fedcba0`
    * tags `--tags|-t Name:my-eni-1,Name:my-eni-2`
    * tag keys `--tag-keys|-k Name,Environment`
    * instance ids `--instance-ids|-I i-1234567890abcdef0,i-0987654321fedcba0`
    * availability zones `--availability-zones|-z a,b`
    * private ips `--private-ips|-p 172.16.0.1,172.17.1.254`
    * public ips `--public-ips|-P 52.28.19.20,52.30.31.32`
  * ENIs doesn't support sort at the moment

And you can combine these filters together.

## Installation

You can download the binary from the [releases](https://github.com/dyegoe/awss/releases) page.

Or you can build it from source:

```txt
git clone http://github.com/dyegoe/awss
cd awss
go build
cp awss /usr/local/bin
```

## Configuration

AWSS uses a configuration file to set the default:

* profiles
* regions
* output format
* show empty results
* show tags
* all regions to search
* sort field for ec2 instances

The default configuration file is `~/.awss/config.yaml` but you can specify another one using `--config` flag.

You can create multiple configuration files and use them with the `--config` flag.

AWSS will search for the configuration file in the following order:

1. `--config` flag absolute path. Either a directory or a file. If it is a directory it will search for the `config.yaml` file.
2. `--config` flag relative path. Either a directory or a file. If it is a directory it will search for the `config.yaml` file.
3. `--config` flag file name. It will search for the file in the current directory or `$HOME/.awss/`
4. `config.yaml` file in the current directory
5. `$HOME/.awss/config.yaml` file

The configuration file is a YAML file with the following structure:

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
  --regions eu-central-1,us-east-1,sa-east-1 \
  ec2 \
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

* [AWS SDK Go v2](https://github.com/aws/aws-sdk-go-v2)
* [Cobra](https://github.com/spf13/cobra)
* [Viper](https://github.com/spf13/viper)
* [Go-Pretty](https://github.com/jedib0t/go-pretty)
* [Go release binaries](https://github.com/marketplace/actions/go-release-binaries)
