---
name: Bug report
about: Something does not work as documented
title: "[BUG] "
labels: bug
assignees: ''

---

**Describe the bug**
A clear and concise description of what is wrong.

**Command**
The exact command you ran (redact account IDs, bucket names or IPs if needed):

```bash
awss --regions us-east-1 ec2 --names 'web-*'
```

**Output**
What awss printed, including the error message if any:

```text

```

**Expected behavior**
What you expected to happen instead.

**Environment**
- awss version (`awss --version`):
- OS and architecture (e.g. Linux amd64, macOS arm64):
- How credentials are provided (named profile, `AWS_PROFILE`, env vars, SSO):
- Config file in use (`~/.awss/config.yaml`), if relevant:

**Additional context**
Anything else that helps, such as whether it worked in a previous version.
