# RELEASE.md

## Release Process

- To release the software, you need to create a new tag or write a CI pipeline. Example command to create a new tag:

```bash
git tag -a v1.0.0 -m "Release version 1.0.0"
git push origin v1.0.0
```


- After creating the tag, please build and push the Dockerfile to a registry:
```bash
DOCKER_REPO=<REGISTRY>/<USERNAME> make docker-push
```

- Once the Docker image is published, you can release the CLI. Currently, we have `.gorelease.yaml` to release the CLI on GitHub, which will also release a Homebrew formula to make it easy to download the binary:

NOTE: Please replace a few variables in the `.goreleaser.yaml` files. Currently, goreleaser points to a dummy repository of my account. Please replace the repository name and GitHub owner details before proceeding.


```bash
export GITHUB_TOKEN
goreleaser release
```

- Once the CLI is released and the Homebrew formula is committed to your repository, you can download the CLI using:
```
brew tap <GITHUB_OWNER>/<GITHUB_REPOSITORY> (brew tap yindia/s3fs)

brew install s3fs
```

- For installation on Linux and Windows, we can generate a bash script from Goreleaser that will provide one-click installation.

**Note:** This has not been implemented because we need a repository setup.