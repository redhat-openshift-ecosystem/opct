# How to Contribute

OPCT projects are [Apache 2.0 licensed](LICENSE) and accept contributions via
GitHub pull requests. This document outlines some of the conventions on
development workflow, commit message formatting, contact points and other
resources to make it easier to get your contribution accepted.

## Code of Conduct

This project and everyone participating in it is governed by the [OPCT Code of
Conduct](./CODE_OF_CONDUCT.md).

## Certificate of Origin

By contributing to this project you agree to the Developer Certificate of
Origin (DCO). This document was created by the Linux Kernel community and is a
simple statement that you, as a contributor, have the legal right to make the
contribution. See the [DCO](DCO) file for details.

## Signing Commits

To contribute to OPCT projects, you must sign the commits. To setup
signing commits see the [Github Guide Signing commits](github-signing-commits).

## Security Response

If you've found a security issue that you'd like to disclose confidentially, please contact Red Hat's Product Security team.
Details [here][security].

## Getting Started

- Read the [Documentation](https://redhat-openshift-ecosystem.github.io/opct)
- Read the [Development Guide](https://redhat-openshift-ecosystem.github.io/opct/devel/guide.md)
  and the [Development FAQ](https://redhat-openshift-ecosystem.github.io/opct/devel/FAQ.md)
  for build and test instructions
- Play with the project, submit documentation updates, bug reports, and patches!

### Contribution Flow

Anyone may [file issues][new-issue].

For contributors who want to work up pull requests, the workflow is roughly:

1. Create a topic branch from where you want to base your work (usually main).
2. Make commits of logical units.
3. Make sure your commit messages are in the proper format (see [below](#commit-message-format)).
4. Push your changes to a topic branch in your fork of the repository.
5. Make sure the tests pass, and add any new tests as appropriate.
6. We run a number of linters and tests on each pull request.
    You may wish to run these locally before submitting your pull request (Make sure you have [podman][podman-install] installed):
    ```sh
    make vet
    make tests
    ```
7. Submit a pull request to the original repository.
8. The [repo](OWNERS) [owners](OWNERS_ALIASES) will respond to your issue promptly, following [the ususal Prow workflow][prow-review].

Thanks for your contributions!

## Coding Style

The coding style suggested by the Golang community is used in OPCT. See the [style doc][golang-style] for details. Please follow them when working on your contributions.

## Commit Message Format

We follow a rough convention for commit messages that is designed to answer two
questions: what changed and why. The subject line should feature the what and
the body of the commit should describe the why.

```
scripts: add the test-cluster command

this uses tmux to set up a test cluster that you can easily kill and
start for debugging.

Fixes #38
```

The format can be described more formally as follows:

```
<subsystem>: <what changed>
<BLANK LINE>
<why this change was made>
<BLANK LINE>
<footer>
```

The first line is the subject and should be no longer than 70 characters, the
second line is always blank, and other lines should be wrapped at 80 characters.
This allows the message to be easier to read on GitHub as well as in various
git tools.


[golang-style]: https://github.com/golang/go/wiki/CodeReviewComments
[new-issue]: https://github.com/redhat-openshift-ecosystem/provider-certification-tool/issues/new
[podman-install]: https://podman.io/getting-started/installation
[prow-review]: https://github.com/kubernetes/community/blob/master/contributors/guide/owners.md#the-code-review-process
[security]: https://access.redhat.com/security/team/contact
[signing-commits]: https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits
