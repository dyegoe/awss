# Security policy

## Supported versions

Only the latest release on the [releases page](https://github.com/dyegoe/awss/releases) receives
fixes. There are no long-term support branches.

## Reporting a vulnerability

Please do not open a public issue for security problems.

Use GitHub's private vulnerability reporting instead: open the
[Security tab](https://github.com/dyegoe/awss/security/advisories/new) of this repository and
click **Report a vulnerability**. You will get an acknowledgement within a week.

Please include the awss version, the command you ran, and what an attacker could gain.

## Scope

awss is a read-only client: it only calls `Describe*`, `List*` and `sts:GetCallerIdentity`.
Credentials are resolved by the AWS SDK's standard chain and are never written to disk or
printed. Reports about the AWS SDK itself should go to
[AWS](https://aws.amazon.com/security/vulnerability-reporting/).
