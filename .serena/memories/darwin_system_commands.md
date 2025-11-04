# Darwin (macOS) System Commands

This document describes system commands and utilities available on Darwin (macOS) systems.

## System Information

The project is developed on **Darwin** (macOS) systems.

## Standard Unix Commands

Darwin is Unix-based (BSD variant), so standard Unix commands work:

### File Operations
```bash
# List files
ls -la                    # Detailed list with hidden files
ls -lh                    # Human-readable sizes

# Copy, move, remove
cp source dest            # Copy file
cp -r source/ dest/       # Copy directory recursively
mv source dest            # Move/rename
rm file                   # Remove file
rm -rf directory/         # Remove directory recursively (use with caution)

# Create directories
mkdir dirname             # Create directory
mkdir -p path/to/nested   # Create nested directories

# View files
cat file                  # Display entire file
less file                 # Paginated view
head -n 20 file          # First 20 lines
tail -n 20 file          # Last 20 lines
tail -f file             # Follow file (useful for logs)
```

### Search and Find
```bash
# Find files
find . -name "*.go"                    # Find Go files
find . -type f -name "launcher.go"     # Find specific file
find . -type d -name "session"         # Find directories

# Search content
grep -r "pattern" .                    # Recursive search
grep -i "pattern" file                 # Case-insensitive
grep -n "pattern" file                 # Show line numbers
grep -l "pattern" *.go                 # List matching files only

# Ripgrep (faster alternative, if installed)
rg "pattern"                           # Fast recursive search
rg -t go "SessionManager"              # Search in Go files only
```

### Text Processing
```bash
# AWK
awk '{print $1}' file                 # Print first column
awk '/pattern/ {print}' file          # Print matching lines

# SED
sed 's/old/new/g' file                # Replace text (preview)
sed -i '' 's/old/new/g' file          # Replace in-place (macOS requires '')
```

### Process Management
```bash
# View processes
ps aux                                 # All running processes
ps aux | grep relay                    # Find specific process
top                                    # Interactive process viewer
htop                                   # Better process viewer (if installed)

# Kill processes
kill PID                               # Graceful termination
kill -9 PID                            # Force kill
killall processname                    # Kill by name
pkill -f "pattern"                     # Kill matching pattern
```

### Networking
```bash
# Network connections
netstat -an                            # Active connections
lsof -i :8080                         # What's using port 8080
nc -zv localhost 8080                  # Test TCP connection

# DNS
nslookup domain.com                    # DNS lookup
dig domain.com                         # Detailed DNS info
```

### System Information
```bash
# System
uname -a                               # System information
sw_vers                                # macOS version
which command                          # Find command location
echo $PATH                             # Show PATH

# Disk usage
df -h                                  # Disk free space
du -sh directory/                      # Directory size
du -h --max-depth=1 .                 # Size of subdirectories
```

### Archives and Compression
```bash
# Tar
tar -czf archive.tar.gz files/         # Create compressed archive
tar -xzf archive.tar.gz                # Extract archive
tar -tzf archive.tar.gz                # List contents

# Zip
zip -r archive.zip files/              # Create zip
unzip archive.zip                      # Extract zip
```

## macOS-Specific Commands

### Clipboard
```bash
# Copy to clipboard
cat file | pbcopy                      # Copy file contents
echo "text" | pbcopy                   # Copy text

# Paste from clipboard
pbpaste                                # Output clipboard contents
pbpaste > file                         # Save clipboard to file
```

### Open Files/Applications
```bash
open file.txt                          # Open with default app
open -a "Visual Studio Code" file      # Open with specific app
open .                                 # Open current directory in Finder
open http://localhost:8080             # Open URL in browser
```

### System Services
```bash
# Launchctl (service management)
launchctl list                         # List services
launchctl start service                # Start service
launchctl stop service                 # Stop service
```

## Docker/Container Commands

Since this project uses Docker (via Packnplay):

```bash
# Docker basics
docker ps                              # Running containers
docker ps -a                           # All containers
docker images                          # List images
docker logs container_id               # View logs
docker logs -f container_id            # Follow logs

# Docker cleanup
docker stop container_id               # Stop container
docker rm container_id                 # Remove container
docker rmi image_id                    # Remove image

# Packnplay-specific
docker ps -a --filter "label=managed-by=packnplay"  # List Packnplay containers
docker ps -a --filter "label=managed-by=packnplay" -q | xargs docker rm -f  # Remove all
```

## Git Commands

```bash
# Basic operations
git status                             # Check status
git add .                              # Stage all changes
git add file                           # Stage specific file
git commit -m "message"                # Commit changes
git push                               # Push to remote
git pull                               # Pull from remote
git log --oneline -10                  # Recent commits

# Branches
git branch                             # List branches
git checkout branch                    # Switch branch
git checkout -b newbranch              # Create and switch
git merge branch                       # Merge branch

# Worktrees (used by Ourocodus)
git worktree list                      # List worktrees
git worktree add path branch           # Add worktree
git worktree remove path               # Remove worktree
```

## Environment Variables

```bash
# View environment
env                                    # All variables
echo $VARIABLE                         # Specific variable
printenv VARIABLE                      # Alternative

# Set temporarily (session only)
export VARIABLE=value

# Set in shell config (~/.zshrc or ~/.bash_profile)
echo 'export VARIABLE=value' >> ~/.zshrc
source ~/.zshrc                        # Reload config
```

## Package Management

```bash
# Homebrew (common on macOS)
brew install package                   # Install package
brew update                            # Update Homebrew
brew upgrade                           # Upgrade packages
brew list                              # List installed packages

# Go modules
go get package@version                 # Get Go package
go install package                     # Install Go binary
go mod tidy                            # Clean dependencies
```

## Useful Combinations

```bash
# Find and delete
find . -name "*.tmp" -delete           # Delete temp files

# Count files
find . -type f | wc -l                 # Count files

# Disk usage sorted
du -sh */ | sort -h                    # Directory sizes sorted

# Recent files
ls -lt | head                          # Recently modified files

# Port usage
lsof -i -P | grep LISTEN               # All listening ports
```

## Notes for AI Agents

- Darwin uses **BSD-style** commands, which can differ slightly from GNU/Linux
- The `-i` flag for `sed` requires an argument on macOS: `sed -i '' 's/old/new/g'`
- Use `pbcopy`/`pbpaste` for clipboard operations (macOS-specific)
- Use `open` command to interact with the GUI (macOS-specific)
- Most standard Unix tools work the same
- Consider using Homebrew for additional utilities
