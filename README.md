# Atun - AWS Tagged Tunnel

SSH tunnel cli tool that works without local configuration. It uses EC2 tags to define hosts and ports forwarding
configuration.
`atun.io/` [schema namespace](#tag-metadata-schema) can be used to configure an SSM tunnel.
![img.png](img.png)

## WIP

This tool is still in development and versions before 1.0.0 might have breaking changes.

## Quickstart

For now, you'd have to build it from scratch. Adding a release process soon.
![demo.gif](demo.gif)

## Features

This tool allows to connect to private resources (RDS, Redis, etc) via EC2 bastion hosts without public IP (via SSM).
At the moment there are only three commands available: `up`, `down`, and `status`.

## Tag Metadata Schema

In order for the tool to work your EC2 host must emply correct tag [schema](schemas/schema.json).
At the moment it has two types of tags: Atun Version and Atun Host.

- **Version** Tag Name = `atun.io/version`
- **Version** Tag Value = `<schema_version>`
- **Env** Tag Name = `atun.io/env`
- **Env** Tag Value = `<environment_name>`
- **Host** Tag Name = `atun.io/host/<hostname>`
- **Host** Tag Value = `{"local":"<local_port>","proto":"<protocol>","remote":<remote_port>}`

### Host Config Description

- local: port that would be bound on a local machine (your computer)
- proto: protocol of forwarding (only `ssm` for now, but might be `k8s` or `cloudflare`)
- remote: port that is available on the internal network to the bastion host.

### Example

| AWS Tag                                                                        | Value                                           | Description                                                               |
|--------------------------------------------------------------------------------|-------------------------------------------------|---------------------------------------------------------------------------|
| `atun.io/version`                                                              | `1`                                             | Schema Version. It might change if significant changes would be intoduced |
| `atun.io/env`                                                                  | `dev`                                           | Specified environment of the bastion host                                 |
| `atun.io/host/nutcorp-api.cluster-xxxxxxxxxxxxxxx.us-east-1.rds.amazonaws.com` | `{"local":"23306","proto":"ssm","remote":3306}` | Describes host config and how to forward ports for a MySQL RDS            |
| `atun.io/host/nutcorp.xxxxxx.0001.use0.cache.amazonaws.com`                    | `{"local":"26379","proto":"ssm","remote":6379}` | Describes host config and how to forward ports for ElastiCache Redis      |

## Usage

### Bring up a tunnel

```bash
atun up
```

### Bring down a tunnel

```bash
atun down
```

### Check the status of the tunnel

```bash
atun status
```
