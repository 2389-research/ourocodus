# Darwin (macOS) System Commands

This file documents system-specific commands and utilities for macOS development.

## System Info
- Current OS: Darwin 24.6.0
- Shell: Bash or Zsh (default on modern macOS)
- Package Manager: Homebrew (brew)

## File Operations

### List Files
```bash
ls -la               # List all files with details
ls -lh               # Human-readable sizes
ls -lt               # Sort by modification time
find . -name "*.go"  # Find files by pattern
```

### File Content
```bash
cat file.txt         # Display entire file
head -n 20 file.txt  # First 20 lines
tail -n 20 file.txt  # Last 20 lines
tail -f file.log     # Follow log file (real-time)
less file.txt        # Page through file
```

### Search
```bash
grep "pattern" file.txt              # Search in file
grep -r "pattern" ./pkg              # Recursive search
grep -i "pattern" file.txt           # Case-insensitive
grep -n "pattern" file.txt           # Show line numbers
rg "pattern"                         # Ripgrep (faster, if installed)
```

### Navigation
```bash
cd /path/to/dir      # Change directory
cd ~                 # Home directory
cd -                 # Previous directory
pwd                  # Print working directory
```

## Process Management

### View Processes
```bash
ps aux               # All running processes
ps aux | grep relay  # Filter processes
top                  # Interactive process viewer
htop                 # Enhanced top (if installed)
```

### Kill Processes
```bash
kill <pid>           # Terminate process by PID
kill -9 <pid>        # Force kill
killall relay        # Kill all processes named "relay"
pkill -f "pattern"   # Kill by pattern
```

### Background Jobs
```bash
command &            # Run in background
jobs                 # List background jobs
fg %1                # Bring job 1 to foreground
bg %1                # Resume job 1 in background
```

## Network

### Ports
```bash
lsof -i :8080        # Check what's using port 8080
lsof -i -P           # List all listening ports
netstat -an | grep 8080  # Alternative port check
```

### Connections
```bash
curl http://localhost:8080/health  # HTTP request
curl -v http://...                 # Verbose output
curl -X POST -d '{}' http://...    # POST request
```

## Docker (macOS-specific)

### Docker Setup
```bash
# Using Docker Desktop
open /Applications/Docker.app

# Using Colima (lightweight alternative)
brew install colima
colima start
colima status
colima stop
```

### Docker Commands
```bash
docker ps                    # Running containers
docker ps -a                 # All containers
docker logs -f <container>   # Follow container logs
docker exec -it <container> /bin/sh  # Shell into container
docker inspect <container>   # Detailed container info
docker system prune          # Clean up unused resources
```

## Git

### Worktree Commands
```bash
git worktree list            # List all worktrees
git worktree add path branch # Create new worktree
git worktree remove path     # Remove worktree
git worktree prune           # Clean up stale worktrees
```

### Branch Management
```bash
git branch -a                # List all branches
git branch -d branch-name    # Delete local branch
git branch -D branch-name    # Force delete local branch
git push origin --delete branch-name  # Delete remote branch
```

### Status and Logs
```bash
git status                   # Working tree status
git log --oneline -10        # Last 10 commits (compact)
git log --graph --oneline    # Visual branch graph
git diff                     # Unstaged changes
git diff --staged            # Staged changes
```

## Environment

### Shell Configuration
```bash
# For Bash
vim ~/.bashrc
source ~/.bashrc

# For Zsh (default on modern macOS)
vim ~/.zshrc
source ~/.zshrc
```

### Environment Variables
```bash
export VAR=value             # Set variable
echo $VAR                    # Print variable
env                          # List all variables
printenv VAR                 # Print specific variable
```

### Path Management
```bash
echo $PATH                   # View PATH
export PATH="/new/path:$PATH"  # Add to PATH
which command                # Find command location
type command                 # Show command type
```

## File Permissions

### View Permissions
```bash
ls -l file.txt               # View file permissions
stat -f "%A" file.txt        # Octal permissions (macOS)
```

### Change Permissions
```bash
chmod +x script.sh           # Make executable
chmod 755 script.sh          # Set permissions (rwxr-xr-x)
chmod -R 755 directory/      # Recursive
```

### Ownership
```bash
chown user:group file.txt    # Change owner
chown -R user:group dir/     # Recursive
```

## Disk and Storage

### Disk Usage
```bash
df -h                        # Disk space (human-readable)
du -sh *                     # Directory sizes
du -h --max-depth=1          # Subdirectory sizes
```

### Find Large Files
```bash
find . -type f -size +100M   # Files larger than 100MB
du -ah . | sort -rh | head -20  # 20 largest items
```

## Compression

### Tar Archives
```bash
tar -czf archive.tar.gz dir/ # Create compressed archive
tar -xzf archive.tar.gz      # Extract archive
tar -tzf archive.tar.gz      # List contents
```

### Zip
```bash
zip -r archive.zip dir/      # Create zip
unzip archive.zip            # Extract zip
unzip -l archive.zip         # List contents
```

## Text Processing

### sed (Stream Editor)
```bash
sed 's/old/new/g' file.txt   # Replace text
sed -i '' 's/old/new/g' file.txt  # In-place replace (macOS)
```

### awk
```bash
awk '{print $1}' file.txt    # Print first column
ps aux | awk '{print $2, $11}'  # Process PID and command
```

### cut
```bash
cut -d',' -f1 file.csv       # First CSV column
echo "a:b:c" | cut -d':' -f2 # Extract field
```

## macOS-Specific

### Clipboard
```bash
pbcopy < file.txt            # Copy file to clipboard
echo "text" | pbcopy         # Copy text to clipboard
pbpaste                      # Paste from clipboard
pbpaste > file.txt           # Save clipboard to file
```

### Open Files
```bash
open file.txt                # Open with default app
open -a TextEdit file.txt    # Open with specific app
open .                       # Open current dir in Finder
open http://localhost:8080   # Open URL in browser
```

### System Info
```bash
uname -a                     # System information
sw_vers                      # macOS version
system_profiler SPSoftwareDataType  # Detailed system info
```

## Package Management (Homebrew)

### Install Packages
```bash
brew install package         # Install package
brew install --cask app      # Install GUI app
brew update                  # Update Homebrew
brew upgrade                 # Upgrade packages
brew list                    # List installed packages
```

### Search and Info
```bash
brew search term             # Search packages
brew info package            # Package information
brew deps package            # Package dependencies
```

### Cleanup
```bash
brew cleanup                 # Remove old versions
brew doctor                  # Check for issues
```

## Useful Aliases

Add these to ~/.zshrc or ~/.bashrc:

```bash
alias ll='ls -lah'
alias la='ls -A'
alias l='ls -CF'
alias ..='cd ..'
alias ...='cd ../..'
alias gst='git status'
alias glog='git log --oneline --graph'
alias dps='docker ps'
alias dlog='docker logs -f'
```
