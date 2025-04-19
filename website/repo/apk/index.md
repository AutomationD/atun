# Alpine Linux Repository

To install atun from our Alpine repository, follow these steps:

1. Add our repository key:
```sh
curl -L https://atun.io/repo/apk/atun@atd.sh-63e7522c.rsa.pub -o /etc/apk/keys/atun@atd.sh-63e7522c.rsa.pub
```

2. Add our repository to your `/etc/apk/repositories`:
```sh
echo "https://atun.io/repo/apk" >> /etc/apk/repositories
```

3. Update your package index and install atun:
```sh
apk update
apk add atun
```

## Repository Structure

This repository is automatically updated with each release. The repository structure follows the standard Alpine repository format:

- `/repo/apk/x86_64/` - Packages for x86_64 architecture
- `/repo/apk/aarch64/` - Packages for ARM64 architecture
- `/repo/apk/APKINDEX.tar.gz` - Package index
