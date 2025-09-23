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

```bash
curl -sSL https://raw.githubusercontent.com/walrus-rclone/mvp/main/install.sh | bash
```

Or download from [releases](https://github.com/walrus-rclone/mvp/releases).

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

Run `walrus-cli web` and open http://localhost:5173 in your browser for a simple drag-and-drop interface.

## Building from Source

```bash
git clone https://github.com/walrus-rclone/mvp.git
cd mvp
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