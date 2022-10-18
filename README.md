# AWSS

AWSS (stands for AWS Search) is a CLI tool to make your life easier when searching AWS resources.
It is a wrapper written in Go using AWS SDK Go v2. The work is still in progress and will be updated regularly.

## Version

The current version is 0.4.0.

## Features

The search runs in parallel using goroutines.

- Search AWS ec2 instances
  - by instance ids
  - by names
  - by tags
  - by private ips
  - by instance types
  - by availability zones
  - by instance states
  - by public ips

And you can combine these filters together.

## Installation

```txt
git clone http://github.com/dyegoe/awss
cd awss
go build
cp awss /usr/local/bin
```

## Usage

```txt
awss --help
awss ec2 --help
awss --profile <profile> --region <region> ec2 --name <name>
```

## Contributing

Contributions are welcome, and they are greatly appreciated! Every little bit helps, and credit will always be given. For major changes, please open an issue first to discuss what you would like to change.

More details in [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache 2.0

## Thanks

- [AWS SDK Go v2](https://github.com/aws/aws-sdk-go-v2)
- [Cobra](https://github.com/spf13/cobra)
- [Go release binaries](https://github.com/marketplace/actions/go-release-binaries)
