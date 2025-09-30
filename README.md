# Walrus CLI

A simple tool to upload, download, and manage files on Walrus storage.

## What it does

- Transfer files from AWS S3 buckets to Walrus
- Upload files to Walrus storage
- Download files back to your computer
- List your stored files
- Estimate storage costs
- Works from command line or web browser

|   |   |   |
|---|---|---|
|<img width="458" height="242" alt="Screenshot 2025-09-29 at 20 45 26" src="https://github.com/user-attachments/assets/a4a74adf-6718-4064-92c1-405bf56ea32d" />|<img width="691" height="520" alt="Screenshot 2025-09-29 at 20 19 27" src="https://github.com/user-attachments/assets/22cf2cfe-38cf-4888-afeb-6f783c1fff5e" />| <img width="1416" height="455" alt="Screenshot 2025-09-30 at 18 20 31" src="https://github.com/user-attachments/assets/13a10486-1141-4b86-8e7e-e494515460cf" /> |
|<img width="1756" height="436" alt="Screenshot 2025-09-29 at 20 49 57" src="https://github.com/user-attachments/assets/045053c3-9aa5-413b-a2b3-4c385137dd64" />|<img width="1407" height="412" alt="Screenshot 2025-09-29 at 20 49 28" src="https://github.com/user-attachments/assets/cdc9799a-bbca-4b40-8a58-74051e72ed39" />|<img width="783" height="731" alt="Screenshot 2025-09-29 at 20 49 18" src="https://github.com/user-attachments/assets/89338e70-dbb7-4bdd-aa71-b4b7dadd61db" />|<img width="1669" height="1015" alt="Screenshot 2025-09-29 at 20 48 48" src="https://github.com/user-attachments/assets/eb50c982-11c3-4d77-985c-f697c036246c" />|
|<img width="1433" height="856" alt="Screenshot 2025-09-30 at 18 20 02" src="https://github.com/user-attachments/assets/d0d5d3bc-dd87-478a-9445-81e8ed198048" />|<img width="639" height="469" alt="Screenshot 2025-09-29 at 20 50 49" src="https://github.com/user-attachments/assets/03b95477-0212-4019-b3bc-544964eb79ca" />|<img width="1455" height="1164" alt="Screenshot 2025-09-29 at 20 50 36" src="https://github.com/user-attachments/assets/1834b55e-e35d-40c4-81dc-e1274ad19401" />|<img width="1418" height="627" alt="Screenshot 2025-09-29 at 20 50 16" src="https://github.com/user-attachments/assets/a797dc82-185b-480f-9859-57ca6a81b41f" />|
|<img width="1417" height="542" alt="Screenshot 2025-09-30 at 18 23 55" src="https://github.com/user-attachments/assets/590599c1-96b9-4d01-9b08-ba7c025a60f8" />|<img width="1445" height="662" alt="Screenshot 2025-09-30 at 18 23 28" src="https://github.com/user-attachments/assets/10d1461b-0f9f-4f29-b0f2-d901b2a3d119" />|<img width="1423" height="590" alt="Screenshot 2025-09-30 at 18 23 18" src="https://github.com/user-attachments/assets/9b37a571-e274-4034-9908-b32ce930f59b" />|

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
