# Quick Start

Get started with Atun in minutes. Follow these simple steps to set up secure tunneling to your AWS resources.

## Installation

### macOS
```bash
brew tap automationd/tap
brew install atun
```

### Alpine Linux
```bash
# Add the repository key
curl -L https://atun.io/repo/apk/atun@atd.sh-63e7522c.rsa.pub -o /etc/apk/keys/atun@atd.sh-63e7522c.rsa.pub

# Add Atun repository
echo "https://atun.io/repo/apk" >> /etc/apk/repositories

# Install Atun
apk update
apk add atun
```

### Windows
```powershell
scoop bucket add automationd https://github.com/automationd/scoop-bucket.git
scoop install atun
```

## Basic Usage
### Start a Tunnel
```bash
atun up
```

### Check Status
```bash
atun status
```

### Stop the Tunnel
```bash
atun down
```

## Next Steps

- Learn about [EC2 Router](/guide/ec2-router)
- Understand the [Tag Schema](/guide/tag-schema)
- Explore [CLI Commands](/reference/cli-commands)
