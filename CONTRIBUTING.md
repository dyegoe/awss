# Contributing

Contributions are welcome, and they are greatly appreciated! Every little bit helps, and credit will always be given. For major changes, please open an issue first to discuss what you would like to change.

You can contribute in many ways:

## Types of Contributions

### Report Bugs

Report bugs at [https://github.com/dyegoe/awss/issues](https://github.com/dyegoe/awss/issues).

If you are reporting a bug, please include:

* Your operating system name and version.
* Any details about your local setup that might be helpful in troubleshooting.
* Detailed steps to reproduce the bug.

### Fix Bugs

Look through the GitHub issues for bugs. Anything tagged with "bug"
is open to whoever wants to implement it.

### Implement Features

Look through the GitHub issues for features. Anything tagged with "feature"
is open to whoever wants to implement it.

### Submit Feedback

The best way to send feedback is to file an issue at [https://github.com/dyegoe/awss/issues](https://github.com/dyegoe/awss/issues).

If you are proposing a feature:

* Explain in detail how it would work.
* Keep the scope as narrow as possible, to make it easier to implement.
* Remember that this is a volunteer-driven project, and that contributions
  are welcome :)

## Get Started

Ready to contribute? Here's how to set up `awss` for local development.

1. Fork the `awss` repo on GitHub.
2. Clone your fork locally:

    ```bash
    git clone git@github.com:your_name_here/awss.git
    ```

3. Setup `pre-commit`:

    ```bash
    sudo apt install pre-commit
    go install golang.org/x/tools/cmd/goimports@latest
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.50.1
    go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
    go install github.com/go-critic/go-critic/cmd/gocritic@latest
    precommit install
    ```

4. Create a branch for local development:

      ```bash
      git checkout -b name-of-your-bugfix-or-feature
      ```

    Now you can make your changes locally. Remember to change the version in `cmd/root.go` and `README.md` files.

5. Commit your changes and push your branch to GitHub::

    ```bash
    git add .
    git commit -m "Your detailed description of your changes."
    git push origin name-of-your-bugfix-or-feature
    ```
  
6. Submit a pull request through the GitHub website.

## Pull Request Guidelines

Before you submit a pull request, check that it meets these guidelines:

1. If the pull request adds functionality, the docs should be updated. Put
   your new functionality into a function with a docstring, and add the
   feature to the list in README.md.

## Source Code Organization

The source code is organized as follows:

* `cmd` - contains the main application entry point. It is responsible for parsing the command line arguments and import the subcommands.
* `logger` - contains the logger implementation.

## Logging

The application uses its own logger wrapper, which is based on the [Zerolog](https://github.com/rs/zerolog) library.

To use the logger, import the `logger` package and create a new logger instance:

```go
import "github.com/dyegoe/awss/logger"

func main() {
    log := logger.NewLogger(logger.DefaultOutput,
        map[string]string{"pkg": "cmd"},
        map[string]string{"cmd": "awss"},
        map[string]string{"func": "persistentPreRunE"},
    )
    log.Info("Hello World!")
}
```

The `NewLogger` function receives two parameters:

* `output` - the output writer. It can be `os.Stdout`, `os.Stderr`, or a file.
* `map[string]string` - a list of key-value pairs that will be added to the log message.
    It can be used to add context to the log message. For example, the `pkg` key can be used to
    add the package name to the log message. It can be empty or `n` key-value pairs.
