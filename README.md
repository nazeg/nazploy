# Server Dashboard

Minimalist server management dashboard for managing multiple web projects, PM2 processes, PocketBase instances, and file management.

## Installation

1. Clone the repository to your Ubuntu server (e.g., `~/nazploy2` or `/opt/dashboard`).
2. Copy `.env.example` to `.env` and modify the values as needed.
3. Run the installation script:

```bash
chmod +x install.sh
./install.sh
```

## Features

- **Project Management:** Create and delete Node.js and Static projects.
- **Service Management:** Start, stop, and restart Node.js projects using PM2.
- **Nginx Configs:** Automatically generates and enables Nginx configurations for projects.
- **File Manager:** Built-in file manager to browse, read, and write files in the `/var/www` directory.
- **PocketBase Integration:** Spin up isolated PocketBase instances for projects on-demand.
- **Logs:** View real-time logs for PM2 processes.

## Usage

Access the dashboard at `http://your-server-ip:3000` (or reverse proxy it to a domain).
Default login is `admin` / `changeme` (configurable in `.env`).

## Note

Ensure you have Nginx installed on your system. PM2 will be installed automatically by the setup script if not present. PocketBase binaries need to be downloaded to `/opt/pocketbase/` if you intend to use the PocketBase feature.
