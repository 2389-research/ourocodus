# Web Asset Compilation

This project uses **Go-native and Rust-native tools** for asset compilation. No Node.js required!

## Tools

- **esbuild** (Go) - TypeScript/JavaScript bundler and minifier
- **minify** (Go) - HTML/CSS/JS minifier
- **Tailwind CLI** (Rust) - CSS framework (optional, install separately)

## Setup

Install all asset compilation tools:

```bash
mise install
```

Check what's installed:

```bash
make assets-check
```

## Project Structure

```
internal/webapp/
├── src/               # Source TypeScript files (if using TS)
│   └── app.ts        # Main TypeScript entry point
├── web/              # Compiled/static assets (embedded in Go binary)
│   ├── app.js        # Compiled JavaScript (from TS or direct)
│   ├── styles.css    # Main stylesheet
│   ├── index.html    # Main HTML
│   └── ...
└── embed.go          # Go embed directive
```

## Development Workflow

### Current Setup (Plain JavaScript + CSS)

The project currently uses plain JavaScript and CSS. Assets are directly in `internal/webapp/web/` and embedded as-is.

**Build:**
```bash
make build          # Compiles assets, then builds Go binaries
```

**Just compile assets:**
```bash
make assets         # Only compiles/minifies web assets
```

### Migrating to TypeScript (Optional)

1. **Create source directory:**
   ```bash
   mkdir -p internal/webapp/src
   ```

2. **Move/create TypeScript files:**
   ```bash
   # Move existing JS or create new TS
   mv internal/webapp/web/app.js internal/webapp/src/app.ts
   ```

3. **Edit TypeScript:**
   ```typescript
   // internal/webapp/src/app.ts
   class RelayConnection {
       private ws: WebSocket | null = null;
       private isConnected: boolean = false;

       constructor() {
           // TypeScript with types!
       }
   }
   ```

4. **Build automatically compiles TS → JS:**
   ```bash
   make build
   # esbuild compiles src/app.ts → web/app.js
   # Go embeds web/app.js into binary
   ```

### Adding Tailwind CSS (Optional)

Tailwind provides a standalone CLI binary (no Node.js):

1. **Download Tailwind CLI:**
   ```bash
   # macOS (Apple Silicon)
   curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-macos-arm64
   chmod +x tailwindcss-macos-arm64
   mv tailwindcss-macos-arm64 /usr/local/bin/tailwindcss

   # macOS (Intel)
   curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-macos-x64
   chmod +x tailwindcss-macos-x64
   mv tailwindcss-macos-x64 /usr/local/bin/tailwindcss

   # Linux
   curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
   chmod +x tailwindcss-linux-x64
   mv tailwindcss-linux-x64 /usr/local/bin/tailwindcss
   ```

2. **Create Tailwind config:**
   ```bash
   cd internal/webapp
   tailwindcss init
   ```

3. **Update Makefile `assets` target to process Tailwind:**
   ```makefile
   assets:
       @# ... existing esbuild stuff ...
       @# Process Tailwind CSS
       @if command -v tailwindcss >/dev/null 2>&1; then \
           echo "  → Processing Tailwind CSS..."; \
           tailwindcss -i internal/webapp/src/styles.css \
                      -o internal/webapp/web/styles.css \
                      --minify; \
       fi
   ```

## Asset Hashing & Cache Busting

The pipeline includes content-based fingerprinting for optimal browser caching:

### Step 4: Asset Fingerprinting

After compilation, assets are hashed using SHA256:

```bash
go run internal/webapp/tools/asset-hash/main.go \
    internal/webapp/web \
    internal/webapp/web/asset-manifest.json
```

This generates:
- Hashed copies: `app.js` → `app.493c9c44.js`
- Manifest mapping original → hashed filenames
- Short 8-character hashes from full SHA256

**Example manifest:**
```json
{
  "app.js": "app.493c9c44.js",
  "logger.js": "logger.54809809.js",
  "styles.min.css": "styles.min.55e70520.css",
  "tailwind.css": "tailwind.fc562ecc.css"
}
```

### Step 5: HTML Injection

The manifest is used to update HTML references:

```bash
go run internal/webapp/tools/inject-hashes/main.go \
    internal/webapp/web/asset-manifest.json \
    internal/webapp/web/index.html
```

**Transformation:**
```html
<!-- Before -->
<link rel="stylesheet" href="styles.min.css">
<script src="app.js"></script>

<!-- After -->
<link rel="stylesheet" href="styles.min.55e70520.css">
<script src="app.493c9c44.js"></script>
```

### Why Hash Assets?

1. **Aggressive Caching** - Browser caches assets forever (filename changes with content)
2. **Cache Busting** - New deploys automatically invalidate old cached assets
3. **CDN Efficiency** - Serve from edge with long TTLs
4. **Parallel Downloads** - Multiple hashed assets download concurrently

### Testing Asset Injection

The inject-hashes tool includes unit tests:

```bash
cd internal/webapp/tools/inject-hashes
go test -v
```

Tests verify:
- HTML attribute preservation (`href=`, `src=`)
- Multiple asset injection
- Query parameter handling
- Already-hashed filename updates
- Regression: no attribute stripping

## Build Pipeline

```
┌─────────────────────┐
│  Source Files       │
│  (src/*.ts)         │
└──────────┬──────────┘
           │
           │ esbuild (TypeScript → JavaScript + bundle + minify)
           ↓
┌─────────────────────┐
│  Compiled Assets    │
│  (web/*.js)         │
└──────────┬──────────┘
           │
           │ minify CSS + Tailwind
           ↓
┌─────────────────────┐
│  Optimized Assets   │
│  (web/*.js, *.css)  │
└──────────┬──────────┘
           │
           │ asset-hash (SHA256 → filename.hash.ext)
           ↓
┌─────────────────────┐
│  Hashed Assets      │
│  (web/*.hash.*)     │
└──────────┬──────────┘
           │
           │ inject-hashes (update HTML references)
           ↓
┌─────────────────────┐
│  Updated HTML       │
│  (index.html)       │
└──────────┬──────────┘
           │
           │ Go embed (//go:embed all:web)
           ↓
┌─────────────────────┐
│  Go Binary          │
│  (bin/relay)        │
└─────────────────────┘
```

## When to Run `make assets`

You **MUST** run `make assets` whenever you:

- ✅ Modify TypeScript files in `internal/webapp/src/`
- ✅ Change CSS in `internal/webapp/web/styles.css` or `tailwind.input.css`
- ✅ Update HTML templates
- ✅ Before committing changes
- ✅ After pulling changes that affect web assets

The asset pipeline automatically:
1. Compiles TypeScript → JavaScript
2. Processes and minifies CSS
3. Generates content hashes
4. Updates HTML with hashed filenames

**Tip:** `make build` includes `make assets`, so running `make build` is sufficient!

## Features

✓ **TypeScript support** - Type-safe JavaScript
✓ **Fast compilation** - esbuild is extremely fast (~100x faster than webpack)
✓ **No Node.js** - Pure Go/Rust toolchain
✓ **Automatic minification** - Smaller binary size
✓ **Content hashing** - SHA256-based cache busting
✓ **Integrated with make** - `make build` handles everything
✓ **Zero config** - Works out of the box
✓ **Tested** - Unit tests verify asset injection correctness

## Benefits

- **Smaller binaries** - Minified assets reduce embedded size
- **Type safety** - TypeScript catches bugs at compile time
- **Modern JavaScript** - Use ES2020+ features, compiled to compatible JS
- **Fast builds** - Go-native tools are blazing fast
- **No npm/node_modules** - Clean Go-only workflow

## Troubleshooting

**Assets not updating?**
```bash
# Clean and rebuild
make clean
make build
```

**Tools not found?**
```bash
# Install/update mise tools
mise install

# Check installation
make assets-check
```

**Want to skip asset compilation during development?**
```bash
# Just build Go binaries (skips asset compilation)
go build -o bin/relay ./cmd/relay
```

## Next Steps

Current state: Plain JavaScript + CSS ✓

To add TypeScript:
1. Create `internal/webapp/src/` directory
2. Move JS to `.ts` files with types
3. Run `make build` - it automatically compiles!

To add Tailwind:
1. Install Tailwind CLI standalone binary
2. Update Makefile `assets` target
3. Add Tailwind directives to CSS
