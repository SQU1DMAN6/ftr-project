# The FtR Project

A comprehensive ecosystem for managing, distributing, and collaboratively editing software packages and file repositories.

**Version:** 3.0  
**Author:** Quan Thai  
**License:** MIT License with Commons Clause condition  
**Copyright:** © 2026 Quan Thai

---

## Table of Contents

1. [Introduction](#introduction)
2. [Project Overview](#project-overview)
3. [Architecture](#architecture)
4. [Core Features](#core-features)
5. [File Structure](#file-structure)
6. [Installation & Setup](#installation--setup)
7. [Usage Examples](#usage-examples)
8. [API Reference](#api-reference)
9. [Technology Stack](#technology-stack)
10. [License](#license)
11. [Contributing & Development](#contributing--development)

---

## Introduction

The **FtR Project** is a comprehensive, open-source ecosystem designed to solve modern software distribution and collaborative development challenges. It consists of two complementary components working in tandem:

- **FtR CLI** - A command-line package manager written in Go that enables downloading, uploading, and managing software packages with automatic multi-language build support
- **Inkdrop** - A backend server providing collaborative file repository management, real-time editing, and secure file transfer capabilities

### Why FtR Exists

Traditional software distribution methods struggle with several critical problems:

1. **Language Fragmentation** - Developers must learn separate tools for Python (pip), Node.js (npm), Go (go get), C++ (manual), etc. FtR unifies all of these under one consistent interface
2. **Build Complexity** - Teams waste time configuring CI/CD pipelines for multi-language projects. FtR auto-detects project types and builds them with zero configuration
3. **Collaborative Friction** - Distributed teams cannot efficiently edit shared files in real-time. Inkdrop provides live collaborative editing comparable to Google Docs but for any file type
4. **Security Concerns** - Sensitive files transmitted over networks are vulnerable. FtR implements client-side AES-256 encryption with key management
5. **Network Unreliability** - Large file uploads fail frequently. FtR's TUS protocol ensures resumable uploads that survive network interruptions
6. **Version Chaos** - Teams struggle tracking which versions are installed and available. FtR maintains a local registry with automatic upgrade detection

Together, these components create a complete solution for:
- **Unified Package Distribution** - One command works across all languages and platforms
- **Collaborative Development** - Real-time file editing with multiple users, presence awareness, and conflict resolution
- **Secure File Transfer** - Client-side AES-256 encryption for sensitive files with intelligent key management
- **Automatic Build Management** - Multi-language build detection and compilation with zero configuration
- **Repository Management** - Centralized file storage with fine-grained permission controls
- **Enterprise-Ready Features** - Session management, audit trails, multi-owner repositories, and permission hierarchies

The project is open-source under a **MIT License with Commons Clause** restriction, allowing free use and modification while preventing commercial redistribution of the software itself.

---

## Project Overview

### FtR - Version 3.0

**Purpose:** A sophisticated CLI package manager for managing file repositories, packages, and build automation.

**Key Characteristics:**
- **Language:** Go 1.24.8
- **CLI Framework:** Cobra
- **Primary Use:** Managing and distributing software packages with integrated build system
- **Architecture:** Command-based CLI with modular package structure

**Core Responsibilities:**
- Download and install packages from Inkdrop repositories
- Upload files and packages to remote repositories
- Pack directories into distributable archives (SQAR/FSDL formats)
- Authenticate users with secure session management
- Automatically detect and compile projects in multiple languages
- Manage local package registry and version tracking
- Search and discover packages in remote repositories
- Encrypt/decrypt sensitive files with AES-256

**Ideal For:**
- Developers managing multi-language projects
- Teams distributing pre-compiled binaries
- Organizations needing secure package distribution
- CI/CD pipelines requiring automated package management

---

### FtR Inkdrop

**Purpose:** A backend web server providing REST APIs for repository management, file operations, and real-time collaborative editing.

**Key Characteristics:**
- **Language:** Go 1.25.6
- **HTTP Framework:** Chi Router v5
- **Database:** SQLite
- **Port:** 6767
- **Primary Use:** Providing server infrastructure for FtR clients and web-based repository access
- **Architecture:** REST API backend with session-based authentication

**Core Responsibilities:**
- User authentication and authorization
- Repository creation, deletion, and browsing
- File and directory operations (create, delete, rename, download)
- Real-time collaborative text editing with multiple users
- Resumable file uploads via TUS protocol
- Repository metadata and permission management
- Static asset serving (CSS, JavaScript)
- Session management and security headers

**Ideal For:**
- Teams needing centralized file repositories
- Organizations requiring collaborative editing
- Projects needing a self-hosted package server
- Teams wanting fine-grained access control

---

## Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    FtR CLI Client                           │
│  (User's local machine - Go binary)                         │
│                                                             │
│  Commands:                                                  │
│  - get (download & build)                                   │
│  - up (upload files)                                        │
│  - down (bulk download)                                     │
│  - pack (create archives)                                   │
│  - login/logout (auth)                                      │
│  - search/list (discovery)                                  │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   │ HTTPS REST API
                   │ (Cookie-based sessions)
                   ▼
┌─────────────────────────────────────────────────────────────┐
│               Inkdrop Backend Server                        │
│  (Runs on port 6767 - Go web application)                   │
│                                                             │
│  Endpoints:                                                 │
│  - /login, /register, /logout (auth)                        │
│  - /browse/{user}/{repo}/* (file browser)                   │
│  - /edit/{file}/{user}/{repo}/* (live editor)               │
│  - /upload/* (TUS resumable upload)                         │
│  - /download (file download)                                │
│  - /new/file, /new/dir, /delete, /rename (ops)              │
│                                                             │
│  Middleware:                                                │
│  - Security headers (CSP, HSTS, X-Frame-Options)            │
│  - Request logging and statistics                           │
│  - Session management (SCS)                                 │
│  - Authentication checks                                    │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   │ SQLite Queries
                   ▼
┌─────────────────────────────────────────────────────────────┐
│              SQLite Database                                │
│  (database.db)                                              │
│                                                             │
│  Tables:                                                    │
│  - users (authentication)                                   │
│  - sessions (active sessions)                               │
│  - repository_meta (repository metadata)                    │
└─────────────────────────────────────────────────────────────┘
                   │
                   │ File System Operations
                   ▼
┌─────────────────────────────────────────────────────────────┐
│           Server File System Storage                        │
│  /srv/ftr/                                                  │
│  ├── userRepositories/                                      │
│  │   ├── {username}/                                        │
│  │   │   ├── {reponame}/  (actual files)                    │
│  │   │   └── ...                                            │
│  │   └── ...                                                │
│  ├── _meta/               (metadata storage)                │
│  │   ├── {username}/                                        │
│  │   │   ├── {reponame}/meta.json                           │
│  │   │   └── ...                                            │
│  │   └── ...                                                │
│  └── tmp/                 (temporary files)                 │
└─────────────────────────────────────────────────────────────┘
```

### Data Flow

**Package Installation Flow (FtR `get` command):**
```
1. User: ftr get user/repo@1.2.3
2. FtR: Authenticate with stored session
3. FtR: Query Inkdrop API for available packages
4. FtR: Download selected package (SQAR or FSDL)
5. FtR: Detect local architecture/OS
6. FtR: Extract archive to local directory
7. FtR: Detect project type (Go, Python, C++, etc.)
8. FtR: Build project using appropriate compiler
9. FtR: Register package in local registry (~/.local/share/ftr/registry.json)
10. FtR: Verify installation with SHA256 hashes
```

**File Upload Flow (FtR `up` command):**
```
1. User: ftr up file.zip user/repo
2. FtR: Authenticate with Inkdrop server
3. FtR: Calculate SHA256 hash of file
4. FtR: Optionally encrypt with AES-256-CBC (-E flag)
5. FtR: Split into chunks (parallel upload, 6 workers)
6. FtR: POST each chunk to Inkdrop /upload endpoint
7. Inkdrop: Store chunks in /srv/ftr/userRepositories/{user}/{repo}/
8. Inkdrop: Update repository metadata
9. FtR: Verify uploaded file integrity
```

**Real-Time Collaboration Flow (Inkdrop live editor):**
```
1. User: Opens /edit/{filename}/{user}/{repo}/path/file.txt
2. Inkdrop: Load file from disk
3. Inkdrop: Establish SSE connection for updates
4. Inkdrop: Track active editors (presence awareness)
5. User A: Types text → Sends diff
6. Inkdrop: Broadcasts update to User B
7. User B: Receives update via SSE → Updates local view
8. Inkdrop: Saves periodically to disk
9. Inkdrop: Maintains version history
```

### Storage Structure

**FtR Client-Side Storage:**
```
~/.config/ftr/
├── session          # Stored session ID
├── email            # Logged-in user email
├── username         # Logged-in username
└── keys/            # Per-file encryption keys
    ├── {filename}.key
    └── ...

~/.local/share/ftr/
└── registry.json    # Local package registry

/tmp/fsdl/           # Temporary extraction directory
```

**Inkdrop Server-Side Storage:**
```
/srv/ftr/
├── userRepositories/          # File storage
│   ├── {username}/            # User's personal repos
│   │   ├── {reponame}/        # Repository files
│   │   │   ├── file1.txt
│   │   │   ├── subdir/
│   │   │   └── ...
│   │   └── ...
│   └── ...
├── _meta/                     # Metadata storage
│   └── {username}/
│       ├── {reponame}/meta.json  # Owners, description, public status
│       └── ...
└── tmp/                       # Temporary files
    ├── uploads/               # TUS upload chunks
    └── archives/              # Generated ZIPs
```

---

## Core Features

### FtR Features

#### 1. **Package Download & Installation**
- Download packages from Inkdrop repositories
- Automatic architecture and OS detection
- Supports version specifications (e.g., `user/repo@1.2.3`)
- Prioritizes platform-specific SQAR packages over universal FSDL
- Automatic project detection and compilation post-download
- Interactive file selection with `-A` flag
- Integrity verification via SHA256 hashing

**Command:** `ftr get [user/repo@version] [options]`

#### 2. **File Upload to Repositories**
- Upload files with optional AES-256 encryption
- Parallel upload (6 concurrent workers)
- Session-based authentication required
- Automatic repository creation (if authorized)
- File integrity verification with SHA256
- Metadata storage with encryption keys

**Command:** `ftr up [file] [user/repo] [options]`

#### 3. **Bulk Download**
- Download all files from a repository at once
- Parallel workers (10 by default)
- Recursive directory support
- ZIP archive support

**Command:** `ftr down [user/repo] [options]`

#### 4. **Package Creation & Packing**
- Pack directories into SQAR or FSDL archives
- Support for platform-specific targeting
- Metadata configuration (BUILD/fsdlbuild.ftr)
- Automatic version tracking

**Command:** `ftr pack [directory] [options]`

#### 5. **Multi-Language Build Detection**
Automatically detects and builds projects in this priority order:
1. Pre-built binaries in `BUILD/linux-{arch}/`
2. Windows MSI in `BUILD/windows/`
3. Custom `BUILD_COMMAND` from metadata
4. Custom `INSTALL_COMMAND` from metadata
5. `install.sh` shell scripts
6. Pre-built ELF binaries
7. `Makefile`
8. Python (`main.py`) via PyInstaller
9. Go (`main.go`) via go build
10. C++ (`main.cpp`) via g++
11. SQU1D (`main.sqd`) via squ1d++

Supported languages:
- Go
- Python
- C++
- Makefile-based projects
- Shell scripts
- SQU1D

#### 6. **Client-Side Encryption**
- AES-256-CBC encryption for uploaded files
- Per-file encryption key storage
- Optional encryption flag (`-E`)
- Key persistence for cross-device decryption
- Plaintext hash verification

**Command:** `ftr up file.zip user/repo -E`

#### 7. **Authentication & Session Management**
- Email or username-based login
- Secure password input (terminal input masking)
- Session persistence in `~/.config/ftr/`
- Cookie-based sessions with server
- Session clearing on logout

**Commands:**
- `ftr login`
- `ftr logout`
- `ftr session`

#### 8. **Search & Discovery**
- Query repositories by name and description
- JSON API integration with Inkdrop
- Fuzzy search capabilities
- Display search results with metadata

**Command:** `ftr search [query]`

#### 9. **Package Registry Management**
- JSON-based local registry
- Track installed packages
- Version management
- Source tracking (user/repo reference)
- Installation metadata

**Registry Location:** `~/.local/share/ftr/registry.json`

#### 10. **Additional Commands**
- `ftr list` - List installed packages and upgradeable versions
- `ftr remove` - Uninstall packages
- `ftr upgrade` - Upgrade installed packages
- `ftr build` - Build projects from source
- `ftr sync` - Sync packages across systems
- `ftr query` - Query package information
- `ftr remote` - Manage remote repositories
- `ftr version` - Display version information
- `ftr clear` - Clear local cache
- `ftr boxlet` - Container/boxlet management

---

### Inkdrop Features

#### 1. **User Authentication & Authorization**
- Registration with email or username
- Secure password hashing (bcrypt)
- Session-based authentication
- Profile management
- Permission controls per repository

**Endpoints:**
- `POST /register` - Create new account
- `POST /login` - Authenticate user
- `POST /logout` - End session
- `GET /sessionconfirm` - Verify active session

#### 2. **Repository Management**
- Create personal repositories
- Delete repositories
- Browse repository file structures
- Set repository visibility (public/private)
- Share repositories with other users
- Repository ownership and multi-owner support

**Endpoints:**
- `POST /new/repo` - Create repository
- `GET /browse/{user}/{reponame}/*` - Browse files
- `DELETE /delete/repo` - Delete repository
- `POST /share` - Share with users

#### 3. **File Operations**
- Create files and directories
- Delete files and directories
- Rename files and directories
- Download individual files
- Download entire directories as ZIP
- Recursive directory listing
- Atomic file writes with temporary files

**Endpoints:**
- `POST /new/file` - Create file
- `POST /new/dir` - Create directory
- `PUT /rename` - Rename item
- `DELETE /delete/item` - Delete file/directory
- `GET /download` - Download files

#### 4. **Real-Time Collaborative Editing**
- Multiple simultaneous users editing same file
- Server-Sent Events (SSE) for live updates
- Presence awareness (who's editing, cursor positions)
- Version tracking and conflict resolution
- Syntax highlighting detection (Ace editor modes)
- Support for multiple file types (Go, JavaScript, Python, Markdown, etc.)
- 5MB file size limit for live editing
- 10-minute idle TTL before editor closes
- Separate document editor for larger files

**Endpoints:**
- `GET /edit/{filename}/{user}/{reponame}/*` - Open editor
- `POST /edit/save` - Save changes
- `GET /edit/updates` - SSE stream for live updates

#### 5. **File Upload via TUS Protocol**
- Resumable uploads for large files
- Chunked upload support
- Automatic retry on network failure
- Cleanup of completed uploads
- Separate TUS endpoint handling

**Endpoints:**
- `OPTIONS/HEAD /upload/*` - Upload negotiation
- `PATCH /upload/*` - Upload chunks
- `POST /upload/*` - Complete upload

#### 6. **Repository Metadata Management**
- Store owners list
- Repository description
- Public/private status
- Timestamps (created, updated)
- Extensible metadata (JSON format)

**Storage:** `/srv/ftr/_meta/{username}/{reponame}/meta.json`

#### 7. **Security Features**
- Content Security Policy (CSP)
- X-Frame-Options (clickjacking protection)
- HSTS (HTTPS enforcement)
- X-Content-Type-Options (MIME sniffing prevention)
- Permissions-Policy (feature restrictions)
- Session cookie security
- Input validation and sanitization

#### 8. **Static Asset Serving**
- CSS and JavaScript delivery
- Ace editor for code highlighting
- Web interface assets
- Optimized loading from CDN when available

#### 9. **Legacy API Compatibility**
- PHP-style routes for backward compatibility
- `/repo.php` endpoints
- `/index.php` support
- `/login.php` redirects

---

## File Structure

### FtR Directory Structure

```
ftr/
├── main.go                      # Entry point
├── go.mod                       # Go module definition (Go 1.24.8)
├── go.sum                       # Dependency checksums
├── install.sh                   # UNIX installation script
├── remove.sh                    # Uninstallation script
│
├── cmd/                         # Command implementations (18 commands)
│   ├── root.go                  # Root command definition
│   ├── commands.go              # Command registration
│   ├── get.go                   # Download and install packages
│   ├── up.go                    # Upload files to repositories
│   ├── down.go                  # Download all files from repository
│   ├── init.go                  # Initialize FtR sync directory
│   ├── pack.go                  # Pack directories into SQAR/FSDL
│   ├── login.go                 # Authenticate to server
│   ├── logout.go                # Clear session
│   ├── search.go                # Search repositories
│   ├── list.go                  # List installed/upgradeable packages
│   ├── version.go               # Display version info
│   ├── build.go                 # Build projects
│   ├── sync.go                  # Sync packages
│   ├── query.go                 # Query information
│   ├── remote.go                # Manage remotes
│   ├── session.go               # Session management
│   ├── upgrade.go               # Upgrade packages
│   ├── clear.go                 # Clear cache
│   ├── remove.go                # Remove packages
│   └── boxlet.go                # Boxlet (container) management
│
├── pkg/                         # Core packages/libraries
│   ├── api/
│   │   └── client.go            # HTTP client for Inkdrop communication
│   ├── builder/
│   │   └── builder.go           # Multi-language build detection
│   ├── boxlet/
│   │   └── meta.go              # Metadata reading/writing
│   ├── fsdl/
│   │   └── package.go           # FSDL format handling (ZIP-based)
│   ├── registry/
│   │   └── registry.go          # Package registry management
│   ├── safety/
│   │   └── patterns.go          # Security patterns
│   ├── screen/
│   │   ├── manager.go           # UI/screen management
│   │   ├── progress.go          # Progress bar rendering
│   │   └── terminal.go          # Terminal utilities
│   └── sqar/
│       └── sqar.go              # SQAR tool detection and invocation
│
└── BUILD/
    └── fsdlbuild.ftr            # FtR metadata configuration
```

### Inkdrop Directory Structure

```
inkdrop/
├── cmd/
│   └── main.go                  # Entry point → boot.BootApp()
│
├── app/
│   ├── boot.go                  # Server initialization and routing
│   ├── middleware.go            # Security headers, logging, stats
│   └── static.go                # Static asset serving
│
├── config/
│   ├── database.go              # SQLite connection setup
│   └── session.go               # Session manager configuration
│
├── controller/                  # HTTP request handlers
│   ├── login/
│   │   └── handler.go           # Login/registration handlers
│   └── repository/
│       ├── browse.go            # Browse, create, delete repos
│       ├── liveedit.go          # Live editor UI handler
│       ├── liveedit_hub.go      # WebSocket/SSE hub for collaboration
│       ├── documentedit.go      # Document editing operations
│       ├── tus.go               # TUS resumable upload handler
│       └── (other handlers)
│
├── model/
│   └── user.go                  # User model, auth, registration
│
├── repository/
│   ├── repository.go            # File/directory operations
│   ├── meta.go                  # Repository metadata handling
│   └── editable_files.go        # File type detection
│
├── view/
│   ├── connector/               # Frontend parameter builders
│   └── template/                # HTML template management
│       └── themes/
│           └── doc-edit/        # Editor theme assets
│
├── routes.go                    # HTTP route definitions
├── database.db                  # SQLite database
├── go.mod / go.sum              # Go module dependencies
├── Makefile                     # Build and deployment
├── inkdrop.service              # Systemd service file
└── assets/
    └── root.css                 # Styling
```

### Key Configuration Files

#### FtR Configuration (`BUILD/fsdlbuild.ftr`)
```
PACKAGE_NAME=FtR
VERSION=3.0.0
TARGET_ARCHITECTURE=all
TARGET_OS=linux
DESCRIPTION=Official FtR manager repository
AUTHOR=Quan Thai
BUILD_COMMAND=go build -o bin/ftr
BUILD_OUTPUT=bin/ftr
INSTALL_COMMAND=install -m 755 bin/ftr /usr/local/bin/ftr
```

#### Inkdrop Service Configuration (`inkdrop.service`)
```ini
[Unit]
Description=Inkdrop File Repository Server
After=network.target

[Service]
Type=simple
WorkingDirectory=/var/www/inkdrop_quan_usr/...
ExecStart=/path/to/inkdrop
Restart=on-failure
User=www-data

[Install]
WantedBy=multi-user.target
```

---

## Installation & Setup

### FtR Installation

#### Linux/Unix

1. **Clone the repository:**
   ```bash
   git clone https://github.com/SQU1DMAN6/ftr-project.git
   cd ftr-project/ftr
   ```

2. **Install dependencies:**
   ```bash
   go mod download
   ```

3. **Run the installation script:**
   ```bash
   chmod +x install.sh
   sudo ./install.sh
   ```

   This installs:
   - Binary: `/usr/local/bin/ftr`
   - Configuration: `~/.config/ftr/`
   - Registry: `~/.local/share/ftr/registry.json`

4. **Verify installation:**
   ```bash
   ftr version
   ```

#### Manual Build

```bash
cd ftr
go build -o bin/ftr ./cmd
./bin/ftr version
```

### Inkdrop Setup

#### Prerequisites
- Go 1.25.6+
- Make
- SQLite3 (included in most systems)

#### Development Setup

1. **Clone the repository:**
   ```bash
   git clone https://github.com/SQU1DMAN6/ftr-project.git
   cd ftr-project/inkdrop
   ```

2. **Install dependencies:**
   ```bash
   go mod download
   go mod tidy
   ```

3. **Run in development mode:**
   ```bash
   make dev
   ```

   The server will start on `http://localhost:6767`

#### Production Build

```bash
make build
make create      # Create bin directory with permissions
make start       # Start via systemd
```

#### Systemd Service Setup

```bash
cd /etc/systemd/system
sudo ln -s /path/to/ftr-project/inkdrop/inkdrop.service .
sudo systemctl daemon-reload
sudo systemctl enable inkdrop
sudo systemctl start inkdrop
sudo systemctl status inkdrop
```

#### Environment Configuration

Create `.env` or set environment variables:
```bash
export FTR_ROOT_DIR=/srv/ftr          # Storage root directory
export FTR_TEST=true                  # Test mode (uses test server)
export PORT=6767                      # HTTP port
```

#### Verify Installation

```bash
# Check server status
curl http://localhost:6767/healthz

# View logs
systemctl logs inkdrop

# Check active routes
make log
```

---

## Usage Examples

### FtR CLI Usage

#### User Authentication
```bash
# Login to Inkdrop server
ftr login
# Prompts for email/username and password
# Stores session in ~/.config/ftr/

# Check current session
ftr session

# Logout
ftr logout
```

#### Searching & Discovery
```bash
# Search for packages
ftr search "golang"

# List installed packages
ftr list

# Check for upgrades
ftr list --upgradeable
```

#### Installing Packages
```bash
# Install latest version
ftr get user/repo

# Install specific version
ftr get user/repo@1.2.3

# Install with auto-selection of files
ftr get user/repo -A

# Install with decryption (if files were encrypted)
ftr get user/repo -D

# Install specific file
ftr get user/repo -f "binary-name"
```

#### Uploading Files
```bash
# Upload file to a repository
ftr up myapp.zip myuser/myrepo

# Upload with encryption
ftr up sensitive-data.zip myuser/myrepo -E

# Upload multiple files
ftr up file1.zip file2.tar.gz myuser/myrepo
```

#### Packaging & Building
```bash
# Pack directory into archive
ftr pack ./my-project

# Build project from source
ftr build ./my-project

# Create SQAR archive
ftr pack ./my-project --format sqar

# Create FSDL archive
ftr pack ./my-project --format fsdl
```

#### Repository Management
```bash
# Download entire repository
ftr down myuser/myrepo

# Remove installed package
ftr remove package-name

# Upgrade all packages
ftr upgrade --all

# Query package information
ftr query myuser/myrepo
```

### Inkdrop Web Interface

#### Accessing the Server
```
http://localhost:6767
```

Default credentials are not provided—users must register. Inkdrop has no default admin account for security reasons.

#### User Registration
1. Navigate to Inkdrop home page
2. Click "Register"
3. Fill in username, email, password
4. Account is immediately active (email verification optional based on config)

#### Creating a Repository
1. Login to Inkdrop
2. Navigate to dashboard
3. Click "New Repository"
4. Name: `myrepo`
5. Description: "My project files"
6. Visibility: Public or Private
7. Click "Create"

#### Uploading Files
1. Navigate to repository
2. Click "Upload Files"
3. Select files or drag-and-drop
4. Wait for TUS upload to complete

#### Collaborative Editing
1. Open a text file in repository
2. Click "Edit" to open live editor
3. Other users can open same file
4. Changes appear in real-time
5. Multiple cursors show other users
6. Auto-saves every 30 seconds

#### Downloading Files
1. Browse repository
2. Click file or directory
3. Click "Download"
4. Receive file or ZIP archive

### Integration Example: FtR with Inkdrop

```bash
# Setup: Start Inkdrop server
cd ftr-project/inkdrop
make dev
# Server runs on localhost:6767

# On another terminal: Login with FtR
ftr login
# Email: user@example.com
# Password: ****

# Upload a project
cd ~/my-project
ftr up . myuser/myrepo

# Another user: Download and build
ftr get myuser/myrepo
# FtR auto-detects project type and builds

# Real-time collaboration
# Open web browser to http://localhost:6767
# Navigate to myuser/myrepo
# Edit file in live editor with team members
```

### Real-World Use Cases

#### Scenario 1: Multi-Language Open Source Project

**Problem:** Your open-source project has components in Go, Python, and C++. Users must run separate build commands for each language, consult documentation, and deal with dependency issues.

**FtR Solution:**
```bash
# Users just run one command
ftr get yourorg/multi-lang-project

# FtR automatically:
# - Detects Go, Python, and C++ components
# - Compiles Go with go build
# - Packages Python with PyInstaller
# - Compiles C++ with g++
# - Places binaries in standard locations
```

#### Scenario 2: Distributed Team Real-Time Collaboration

**Problem:** Your team needs to edit configuration files, documentation, and scripts together. Email attachments and Git commits are too slow for real-time feedback.

**FtR Solution:**
```bash
# Team member 1: Opens Inkdrop web UI
http://localhost:6767/edit/deploy.yaml/john/devops

# Team member 2: Opens same file in browser
# They see live updates as each other types
# Multiple cursors show who's editing what
# Changes auto-save every 30 seconds
```

#### Scenario 3: Secure Binary Distribution

**Problem:** Your company distributes proprietary tools to 100 developers. Email is insecure, SSH distribution is tedious, and you need to verify integrity.

**FtR Solution:**
```bash
# Company: Build and upload with encryption
ftr pack ./myapp -f linux-x86_64
ftr up ./myapp.sqar company/internal-tools -E  # -E flag encrypts

# Developer: Download decrypts automatically
ftr get company/internal-tools
# FtR verifies SHA256 hash automatically
# Optional: -D flag decrypts if encrypted
```

#### Scenario 4: CI/CD Pipeline Integration

**Problem:** Your CI/CD pipeline needs to upload build artifacts to multiple environments (dev, staging, prod) and track versions.

**FtR Solution:**
```bash
# In your GitHub Actions workflow
- name: Upload to FtR
  run: |
    ftr login --token ${{ secrets.FTR_TOKEN }}
    ftr pack ./dist
    ftr up ./dist.sqar myorg/app@${{ github.ref }}
    ftr up ./dist.sqar myorg/app-staging  # Latest tag
```

#### Scenario 5: Knowledge Base with Collaborative Editing

**Problem:** Your team maintains technical documentation, runbooks, and architecture diagrams. Word documents and wikis become outdated quickly because editing is painful.

**FtR Solution:**
- Store all docs in Inkdrop repository
- Multiple team members edit Markdown simultaneously
- Real-time updates—no merge conflicts
- FtR's built-in Ace editor provides syntax highlighting
- Version history maintained automatically

---

## API Reference

### FtR Commands Summary

| Command | Purpose | Syntax |
|---------|---------|--------|
| `get` | Download and install packages | `ftr get [user/repo@version]` |
| `up` | Upload files to repository | `ftr up [file] [user/repo]` |
| `down` | Bulk download repository | `ftr down [user/repo]` |
| `pack` | Create SQAR/FSDL archives | `ftr pack [directory]` |
| `login` | Authenticate with server | `ftr login` |
| `logout` | Clear session | `ftr logout` |
| `search` | Search repositories | `ftr search [query]` |
| `list` | List installed packages | `ftr list [options]` |
| `version` | Display version info | `ftr version` |
| `build` | Build projects | `ftr build [path]` |
| `sync` | Sync packages | `ftr sync` |
| `query` | Query package info | `ftr query [user/repo]` |
| `remote` | Manage remotes | `ftr remote [subcommand]` |
| `session` | Show session info | `ftr session` |
| `upgrade` | Upgrade packages | `ftr upgrade [package]` |
| `clear` | Clear cache | `ftr clear` |
| `remove` | Remove package | `ftr remove [package]` |
| `boxlet` | Container management | `ftr boxlet [subcommand]` |

### Inkdrop Endpoints Summary

| Method | Endpoint | Purpose |
|--------|----------|---------|
| `POST` | `/register` | Register new user |
| `POST` | `/login` | Authenticate user |
| `POST` | `/logout` | End session |
| `GET` | `/sessionconfirm` | Verify session |
| `GET` | `/browse/{user}/{repo}/*` | Browse repository |
| `GET` | `/edit/{file}/{user}/{repo}/*` | Open live editor |
| `POST` | `/new/file` | Create file |
| `POST` | `/new/dir` | Create directory |
| `PUT` | `/rename` | Rename item |
| `DELETE` | `/delete/item` | Delete file/dir |
| `GET` | `/download` | Download files |
| `POST` | `/upload/*` | TUS file upload |
| `GET` | `/healthz` | Health check |
| `GET` | `/preview` | File preview |

---

## Technology Stack

### FtR Technologies

**Language & Runtime:**
- Go 1.24.8
- Linux-focused with cross-platform awareness

**Direct Dependencies:**
- `github.com/spf13/cobra` v1.10.1 - CLI framework
- `golang.org/x/term` v0.36.0 - Terminal I/O

**Standard Library (Key Usage):**
- `archive/zip` - FSDL format (ZIP-based archives)
- `crypto/aes`, `crypto/cipher` - AES-256-CBC encryption
- `crypto/sha256` - File integrity verification
- `net/http`, `net/http/cookiejar` - HTTP client, session management
- `encoding/json` - API communication, registry format
- `os/exec` - Execute build commands and external tools

**External Tools Integration:**
- InkDrop Server (API calls via HTTP)
- SQAR Tool (optional compression)
- Build Tools: Go, Python/PyInstaller, Make, g++, squ1d++

### Inkdrop Technologies

**Language & Runtime:**
- Go 1.25.6
- Linux native

**HTTP & Routing:**
- `github.com/go-chi/chi/v5` - Modern HTTP router

**Database:**
- `github.com/uptrace/bun` - SQL query builder
- `github.com/uptrace/bun/dialect/sqlitedialect` - SQLite dialect
- `github.com/uptrace/bun/driver/sqliteshim` - SQLite driver
- SQLite3 database

**File Upload:**
- `github.com/tus/tusd/v2` - TUS protocol for resumable uploads

**Security:**
- `golang.org/x/crypto` - Password hashing (bcrypt)

**Session Management:**
- `github.com/alexedwards/scs/v2` - Secure session cookie manager

**Frontend:**
- Ace Editor (via CDN) - Code highlighting and editing
- Custom HTML templates - Server-rendered UI
- CSS - root.css for styling

### Shared Architecture Patterns

- **REST API** - FtR and Inkdrop communicate via HTTP/HTTPS
- **JSON** - Data serialization format
- **SQLite** - Local data persistence
- **AES-256-CBC** - Strong encryption standard
- **SHA256** - File integrity verification
- **bcrypt** - Password hashing
- **Cookie-based Sessions** - Stateful authentication

---

## License

### MIT License with Commons Clause Condition

The FtR Project is dual-licensed under:

1. **MIT License** - Grants broad rights to use, modify, and distribute the software
2. **Commons Clause** - Restricts commercial redistribution of the software itself

**Summary:**

Allowed:
- Use FtR for personal projects
- Modify and improve the code
- Use FtR in commercial products (for internal tools)

Not Allowed:
- Sell the FtR software itself
- Offer FtR as a commercial service
- Create competing products based directly on FtR

**Full License:** See `LICENSE` file in repository

**Copyright:** © 2026 Quan Thai

---

## Contributing & Development

### Building from Source

#### FtR
```bash
cd ftr-project/ftr
go mod download
go build -o bin/ftr ./cmd
./bin/ftr version
```

#### Inkdrop
```bash
cd ftr-project/inkdrop
go mod download
go build -o bin/inkdrop ./cmd
./bin/inkdrop
```

### Development Make Targets

**Inkdrop Makefile:**
```makefile
make dev         # Run in development mode (go run)
make build       # Build optimized binary
make create      # Create bin directory
make reload      # Reload systemd daemon
make start       # Start systemd service
make stop        # Stop systemd service
make restart     # Restart systemd service
make status      # Check service status
make log         # Tail service logs
make update      # Pull + build + reload + restart + log
```

### Running Tests

```bash
# FtR tests
cd ftr
go test ./...

# Inkdrop tests
cd inkdrop
go test ./...
```

### Database Setup

Inkdrop uses SQLite with automatic schema creation:

```bash
cd inkdrop
rm database.db            # Reset database
./bin/inkdrop             # Creates fresh schema
```

Database tables are auto-created by Inkdrop on first run.

### Debugging

**FtR Debug Mode:**
```bash
FTR_DEBUG=true ftr get user/repo
```

**Inkdrop Debug Mode:**
```bash
INKDROP_DEBUG=true ./bin/inkdrop
make log                           # View systemd logs
```

### Code Organization Best Practices

**FtR:**
- Commands in `cmd/` directory
- Shared logic in `pkg/` packages
- Clear separation: CLI logic vs. business logic
- Use `pkg/api/` for server communication
- Use `pkg/builder/` for project detection

**Inkdrop:**
- HTTP handlers in `controller/` directories
- Data models in `model/`
- Storage operations in `repository/`
- Routes defined in `routes.go`
- Middleware in `app/middleware.go`

### Contributing Guidelines

1. Fork the repository
2. Create feature branch: `git checkout -b feature/my-feature`
3. Make changes with clear commit messages
4. Test thoroughly: `go test ./...`
5. Submit pull request with description
6. Code review by maintainers

### Reporting Issues

Report bugs and feature requests on GitHub Issues with:
- Clear description of issue
- Steps to reproduce
- Expected vs. actual behavior
- System information (OS, Go version)
- Relevant logs or error messages

---

## Summary

The **FtR Project** represents a comprehensive solution for:
- **Distributed package management** with automatic build support
- **Secure file transfer** with client-side encryption
- **Collaborative development** with real-time editing
- **Multi-language project support** with automatic build detection
- **Enterprise-grade security** with encryption, authentication, and access control

**FtR CLI** provides developers with powerful tools to manage and distribute packages, while **Inkdrop** provides the backend infrastructure for repository management, file operations, and collaborative editing.

Together, they create a modern, efficient, and secure package distribution and collaboration platform.

---

**For more information:**
- Repository: https://github.com/SQU1DMAN6/ftr-project
- Issues: https://github.com/SQU1DMAN6/ftr-project/issues
- Author: Quan Thai
- Version: 3.0
- License: MIT License with Commons Clause condition
