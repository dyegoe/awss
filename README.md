# AWSS

AWSS (stands for AWS Search) is a CLI tool to make your life easier when searching AWS resources.
It is a wrapper written in Go using AWS SDK Go v2. The work is still in progress and will be updated regularly.

## Features

- Search AWS ec2 instances
  - by name
  - by tag
  - by instance id
  - by private ip
  - by public ip

## Installation

```bash
git clone http://github.com/dyegoe/awss
cd awss
go build
cp awss /usr/local/bin
```

## Usage

```bash
awss --help
awss ec2 --help
awss --profile <profile> --region <region> ec2 --name <name>
```

## Contributing

Contributions are welcome, and they are greatly appreciated! Every little bit helps, and credit will always be given. For major changes, please open an issue first to discuss what you would like to change.

More details in [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache 2.0
