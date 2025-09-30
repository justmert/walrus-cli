# Walrus CLI

A simple tool to upload, download, and manage files on Walrus storage.

## What it does

- Transfer files from AWS S3 buckets to Walrus
- Upload files to Walrus storage
- Download files back to your computer
- List your stored files
- Estimate storage costs
- Works from command line or web browser

## Installation

### Using the install script (macOS/Linux)
```bash
curl -sSL https://raw.githubusercontent.com/justmert/walrus-cli/master/install.sh | bash
```

### Using Go
```bash
go install github.com/justmert/walrus-cli/cmd/walrus-cli@latest
```

### Manual download
Download pre-built binaries from [releases](https://github.com/justmert/walrus-cli/releases).

## Getting Started

First, set up your configuration:

```bash
walrus-cli setup
```

Then you can:

```bash
# Upload a file
walrus-cli upload myfile.pdf

# List your files
walrus-cli list

# Download a file
walrus-cli download myfile.pdf

# Open web interface
walrus-cli web
```

## Web Interface

Run `walrus-cli web` and open http://localhost:5173 in your browser for an interface.

### S3 Transfer

Transfer files from AWS S3 to Walrus storage directly from the web interface.

#### Getting AWS Credentials

For enhanced security, use temporary session tokens:

```bash
# Generate temporary credentials (valid for 1 hour)
aws sts get-session-token --duration-seconds 3600
```

Use the returned `AccessKeyId`, `SecretAccessKey`, and `SessionToken` in the web interface.

## Building from Source

```bash
git clone https://github.com/justmert/walrus-cli.git
cd walrus-cli
make build
```

## Configuration

Config file: `~/.walrus-rclone/config.yaml`

```yaml
walrus:
  aggregator_url: "https://aggregator.walrus-testnet.walrus.space"
  publisher_url: "https://publisher.walrus-testnet.walrus.space"
  epochs: 5
```

## License

MIT