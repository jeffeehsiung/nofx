# 🚀 NOFX+ Complete Setup Guide

Everything you need to set up NOFX+ is in this file. No need to jump around!

> ⚠️ **Important Note**: 
> - **Local Setup (`./setup.sh`)** → Installs **NOFX+ (nofxplus)** with all enhancements
> - **Docker Deployment (`install.sh`)** → Runs **original NOFX** from official repo
>
> **Choose your path:**
> - Want NOFX+ features? → Use `./setup.sh` (local development)
> - Want official NOFX with Docker? → Use `install.sh` (one-click Docker deployment)

## ⚡ Quick Start (Most Users)

```bash
./setup.sh
```

That's it! The script will:
- ✅ Check your system
- ✅ Install TA-Lib (with smart fallbacks)
- ✅ Download all dependencies
- ✅ Generate encryption keys
- ✅ Open browser for API keys
- ✅ Build your project
- ✅ Verify everything works

**Time: 5-10 minutes**

Then start your app:
```bash
# Terminal 1: Backend
make run

# Terminal 2: Frontend (new terminal)
make run-frontend

# Open browser
http://localhost:3000
```

---

## � Available Scripts - Choose What You Need

### **For Local Development (Most Users)**

#### `./setup.sh` (macOS/Linux)
**What it does:**
- Checks prerequisites (Go, Node.js)
- Installs TA-Lib with smart fallbacks
- Downloads dependencies
- Generates encryption keys
- Configures API keys (interactive, browser-guided)
- Builds backend and frontend
- Verifies installation

**When to use:**
- First time setup on your computer
- Local development environment

**Usage:**
```bash
./setup.sh
```

**Also use for managing API keys later:**
```bash
./setup.sh --api-keys
```
Menu options:
- Configure DeepSeek
- Configure Binance, Bybit, Kraken, Coinbase, Kucoin
- Configure OpenAI (optional)
- View current configuration

---

### **For Docker Deployment (Original NOFX from Official Repo) 🐳**

#### `./install.sh` (Automatic One-Click Docker Setup) - **ORIGINAL NOFX VERSION**
**What it does:**
- Checks Docker installation
- Creates installation directory
- Downloads Docker Compose configuration from official NOFX repo
- Generates encryption keys automatically
- Pulls Docker images
- Starts all services
- Displays access URLs

**When to use:**
- One-click production deployment
- Server/cloud environments
- Docker-based setup (no local Go/Node.js needed)
- **Running original NOFX** (not NOFX+ enhancements)

**Important:** This deploys the **original NOFX** from the official repository, not the NOFX+ version with all the enhancements

**Usage:**
```bash
# Download and run (recommended)
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash

# Or with custom directory
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash -s -- /opt/nofx
```

**Benefits:**
- Single command installation
- Automatic Docker setup
- No system dependencies needed
- Self-contained deployment

---

## 🎯 Quick Comparison: Which Setup Should You Use?

| Feature | Local Setup (`./setup.sh`) | Docker Setup (`install.sh`) |
|---------|---------------------------|---------------------------|
| **Version** | ✅ NOFX+ (Enhanced) | ⚠️ Original NOFX |
| **Feedback Loops** | ✅ Yes | ❌ No |
| **Prompt Evolution** | ✅ Yes | ❌ No |
| **Threshold Calibration** | ✅ Yes | ❌ No |
| **Setup Time** | ~5-10 min | ~2-5 min |
| **System Requirements** | Go 1.21+, Node.js 18+ | Docker only |
| **Best For** | Developers, NOFX+ features | Quick deployment |
| **Recommended** | ⭐ Yes | For original NOFX only |

**Choose:** Want NOFX+ enhancements? Use local setup. Want original NOFX? Use Docker.

---

### **For Windows Users**

#### `setup.bat` (Windows Setup) - **NOFX+ VERSION**
**What it does:**
- Checks prerequisites (Git, Go, Node.js)
- Installs Go and npm dependencies
- Creates .env file
- Builds backend and frontend

**When to use:**
- Windows 10+ development environment
- PowerShell or Command Prompt

**Usage:**
```cmd
setup.bat
```

---

### **For Docker Management**

#### `./start.sh` (Docker Helper Script)
**What it does:**
- Manage Docker containers easily
- View logs
- Check service status
- Restart services
- Clear trading data

**When to use:**
- You deployed with `install.sh`
- Need to manage running Docker services

**Commands:**
```bash
./start.sh start          # Start services
./start.sh stop           # Stop services
./start.sh restart        # Restart services
./start.sh logs           # View logs
./start.sh status         # Check status
./start.sh clean          # Clean all containers
./start.sh regenerate-keys # Regenerate encryption keys
```

---

### **Optional: Advanced Tools**

#### `./verify.sh` (Installation Verification)
**What it does:**
- Checks all prerequisites
- Verifies dependencies installed
- Checks build artifacts
- Tests port availability
- Shows detailed status report

**When to use:**
- Troubleshooting setup issues
- Verify everything is installed

**Usage:**
```bash
./verify.sh
```

---

## �📋 What You Need Before Starting

Make sure you have installed:

- **Go 1.21 or higher** → [Download](https://golang.org/dl/)
- **Node.js 18 or higher** → [Download](https://nodejs.org/)
- **Git** → [Download](https://git-scm.com/)

That's all! The setup script handles everything else.

---

## 🔄 Step-by-Step Walkthrough

### Step 1: Clone the Repository

```bash
git clone https://github.com/jeffeehsiung/nofxplus.git
cd nofxplus
```

### Step 2: Run the Setup Script

```bash
./setup.sh
```

You'll see output like this:
```
===================================================
Step 1: Checking System Prerequisites
===================================================
ℹ Detected OS: macos
✓ Go 1.21.0 found
✓ Node.js 18.15.0 found

===================================================
Step 2: Installing TA-Lib (Technical Indicator Library)
===================================================
ℹ Trying Homebrew installation...
✓ TA-Lib installed via Homebrew
```

### Step 3: When Browser Opens

The script will open **two browser windows**:

**1. DeepSeek API (Required for AI features)**
- Website opens: https://platform.deepseek.com/api_keys
- Sign up or log in
- Create new API key
- Copy the key
- Go back to terminal and paste when asked

**2. Binance API (Required for trading)**
- Website opens: https://www.binance.com/en
- Log in (or sign up)
- Click Account → API Management
- Create or find your API key
- Copy API Key and Secret
- Go back to terminal and paste when asked

### Step 4: Enter API Keys

When prompted in terminal:
```
Enter your DeepSeek API Key: [paste here]
Enter your Binance API Key: [paste here]
Enter your Binance API Secret: [paste here - won't be visible]
```

### Step 5: Wait for Build

The script automatically:
```
Building backend...
Building frontend...
Verifying installation...
```

### Step 6: Script Completes

You'll see:
```
===================================================
Setup Complete! 🎉
===================================================
✓ Backend built successfully
✓ Frontend built successfully
✓ Installation verification complete!
```

### Step 7: Start the Application

**Terminal 1:**
```bash
make run
# or
./nofx
```

**Terminal 2 (new):**
```bash
make run-frontend
# or
cd web && npm run dev
```

**Open in browser:**
```
http://localhost:3000
```

Done! ✅

---

## 🔐 Understanding the API Keys

### DeepSeek API (REQUIRED)

**What is it?**
- AI platform for trading analysis
- Provides intelligent trading recommendations
- Uses natural language processing
- Helps make better trading decisions

**Getting your API key:**
1. Visit https://platform.deepseek.com/api_keys
2. Sign up or log in
3. Create a new API key
4. Copy it (format: `sk-...`)

**In the setup:**
- Browser opens automatically to the page
- Just sign up, copy your key, paste in terminal
- Script saves it securely in `.env`

**If you don't have it yet:**
- Click the link when prompted
- Take 5 minutes to sign up
- Create a free account
- Generate your first API key

### Binance API (REQUIRED)

**What is it?**
- Trading exchange connection
- Allows the bot to place trades
- Needs both API Key and Secret

**Getting your API keys:**
1. Visit https://www.binance.com/en
2. Log in (sign up if needed)
3. Click **Account** (top right)
4. Select **API Management**
5. Create new API key
6. Copy both "API Key" and "API Secret"

**Special considerations:**
- **Demo Trading?** Use https://demo.binance.com/en/my/settings/api-management instead
  - This uses play money, not real funds
  - Great for testing before live trading
  - Copy demo API keys into setup

- **Testnet?** Use https://testnet.binance.vision/
  - Another safe testing option
  - No real money involved

**In the setup:**
- Browser opens to main Binance site
- You log in and navigate to API Management
- Copy both pieces of information
- Paste in terminal when asked

### Optional APIs

You can add these later:

- **Bybit** - Another trading exchange
- **Kraken** - Crypto trading
- **Coinbase** - US-based crypto
- **Kucoin** - Alternative exchange
- **OpenAI** - Alternative to DeepSeek

To add later:
```bash
./configure-keys.sh
```

---

## 🛠️ Troubleshooting

### Build Errors

#### "Cannot find module 'franc'"
```bash
cd web
npm install franc
npm run build
```

#### "npm ERR! Unsupported engine"
This is just a warning. Your version of Node.js is fine.
Script continues automatically.

#### Frontend build fails
```bash
cd web
npm cache clean --force
rm -rf node_modules package-lock.json
npm install
npm run build
```

#### Backend build fails
```bash
go mod tidy
go mod download
go build -o nofx
```

---

### Go Module Issues

#### "no modules specified" or "go mod download" fails

**Problem:** `go.sum` is missing or invalid

**Solution:**
```bash
# Regenerate go.sum
go mod tidy

# Then download dependencies
go mod download

# Now build
go build -o nofx
```

The `go mod tidy` command will:
- Download all dependencies in `go.mod`
- Create/update `go.sum` with checksums
- Remove any unused dependencies
- Validate the module structure

**If you still get errors:**
1. Verify `go.mod` exists and has valid syntax
2. Check your Go version: `go version` (need 1.21+)
3. Clear the cache: `go clean -modcache`
4. Try again: `go mod tidy && go mod download`

#### "missing go.sum"

If `go.sum` file doesn't exist after `go mod tidy`:
```bash
# Clear mod cache and retry
go clean -modcache
go mod tidy
```

The file should now be created automatically.

### Installation Errors

#### TA-Lib installation fails

**What happens:**
1. Script tries Homebrew (macOS) ✓ Works for most
2. Script tries apt/yum (Linux) ✓ Works for Linux
3. Script tries source compilation ✓ Fallback
4. Script tells you manual steps ✓ Last resort

**Manual installation:**
```bash
# macOS
brew install ta-lib

# Ubuntu/Debian
sudo apt-get update
sudo apt-get install libta-lib0-dev libta-lib0

# CentOS/RHEL
sudo yum install ta-lib ta-lib-devel

# Arch Linux
sudo pacman -S ta-lib
```

Then run setup again:
```bash
./setup.sh
```

#### Go not installed
```bash
# macOS
brew install go

# Ubuntu
sudo apt-get install golang-go

# Or download from: https://golang.org/dl/
```

Then run setup:
```bash
./setup.sh
```

#### Node.js not installed
```bash
# macOS
brew install node

# Ubuntu
sudo apt-get install nodejs npm

# Or download from: https://nodejs.org/
```

Then run setup:
```bash
./setup.sh
```

---

### Port Already in Use

If you get "Address already in use" for port 3000 or 8080:

**Option 1: Change ports in .env**
```
# Edit .env
NOFX_BACKEND_PORT=8081
NOFX_FRONTEND_PORT=3001
```

**Option 2: Kill the process**
```bash
# Find what's using port 3000
lsof -i :3000

# Kill it
kill -9 <PID>
```

---

### API Key Issues

#### DeepSeek page won't load
1. Try manual: https://platform.deepseek.com/api_keys
2. Make sure you're signed in
3. Create a new API key if needed

#### Binance page shows "404 Not Found"
1. First, go to https://www.binance.com/en
2. Log in
3. Then manually go to Account → API Management
4. It should work once you're logged in

#### "Invalid API key" error
1. Check you copied the entire key (no spaces)
2. Verify in your exchange account the key is still active
3. Try creating a new API key
4. Update with:
   ```bash
   ./configure-keys.sh
   ```

#### Accidentally pasted wrong key
1. Edit `.env` file:
   ```bash
   nano .env
   ```
2. Find the line, paste correct key
3. Save (Ctrl+X, Y, Enter)
4. Restart backend:
   ```bash
   make run
   ```

---

## 📁 Configuration Files

### .env.example
- Template file (read-only)
- Shows all available options
- Don't edit this

### .env
- Your actual configuration
- Created by setup script
- **Don't commit to git!** (already in .gitignore)
- Safe to edit manually

**Key variables:**
```
# Server ports
NOFX_BACKEND_PORT=8080
NOFX_FRONTEND_PORT=3000

# Encryption (auto-generated)
JWT_SECRET=...
DATA_ENCRYPTION_KEY=...
RSA_PRIVATE_KEY=...

# API Keys (you add these)
DEEPSEEK_API_KEY=sk-...
BINANCE_API_KEY=...
BINANCE_API_SECRET=...
```

---

## 🔄 After Setup: Managing API Keys

### Update API Keys Later

If you need to add or change API keys:

```bash
./configure-keys.sh
```

Menu options:
```
1) DeepSeek (LLM - Required)
2) Binance (Trading)
3) Bybit (Trading)
4) Kraken (Trading)
5) Coinbase (Trading)
6) Kucoin (Trading)
7) OpenAI (Optional LLM)
8) View current configuration
9) Exit
```

Select the exchange, paste your key, done!

### Check Setup Status

To verify everything is installed correctly:

```bash
./verify.sh
```

Shows:
```
✓ Go 1.21.0 found
✓ Node.js 18.15.0 found
✓ TA-Lib installed
✓ npm dependencies installed
✓ Go modules downloaded
✓ .env file exists
✓ Backend binary built
✓ Frontend built
```

---

## 🏃 Quick Commands Reference

```bash
# Setup (one time)
./setup.sh

# Start backend
make run

# Start frontend (new terminal)
make run-frontend

# Rebuild backend only
make build

# Rebuild frontend only
make build-frontend

# Manage API keys
./configure-keys.sh

# Check setup status
./verify.sh

# Clean build files
make clean
```

---

## 🔧 All Make Commands Available

The Makefile provides many useful development commands:

### **Running the Application**

| Command | Purpose |
|---------|---------|
| `make run` | Start backend server (development mode) |
| `make run-frontend` | Start frontend dev server with hot reload |

### **Building**

| Command | Purpose |
|---------|---------|
| `make build` | Build backend binary (`./nofx`) |
| `make build-frontend` | Build frontend for production (`web/dist/`) |

### **Testing**

| Command | Purpose |
|---------|---------|
| `make test` | Run all tests (backend + frontend) |
| `make test-backend` | Run backend tests only |
| `make test-frontend` | Run frontend tests only |
| `make test-coverage` | Generate backend code coverage report |

**View coverage:**
```bash
make test-coverage
# Opens coverage.html in browser (check your terminal)
```

### **Code Quality**

| Command | Purpose |
|---------|---------|
| `make fmt` | Format all Go code with `gofmt` |
| `make lint` | Run Go linter (requires `golangci-lint`) |

**Install linter (optional):**
```bash
brew install golangci-lint  # macOS
# or
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

### **Dependencies**

| Command | Purpose |
|---------|---------|
| `make deps` | Download Go dependencies |
| `make deps-update` | Update all Go dependencies to latest |
| `make deps-frontend` | Install frontend (npm) dependencies |

### **Docker**

| Command | Purpose |
|---------|---------|
| `make docker-build` | Build Docker images |
| `make docker-up` | Start Docker containers (`-d` = detached) |
| `make docker-down` | Stop and remove Docker containers |
| `make docker-logs` | View Docker container logs (follow mode) |

### **Cleanup**

| Command | Purpose |
|---------|---------|
| `make clean` | Remove build artifacts and test cache |

**What gets cleaned:**
- `./nofx` (backend binary)
- `coverage.out` and `coverage.html`
- `web/dist/` (frontend build)
- Test cache

### **Help**

| Command | Purpose |
|---------|---------|
| `make help` | Show all available commands |

---

## 🛠️ Common Development Workflows

### **Local Development (Full Stack)**

```bash
# Terminal 1: Backend
make run

# Terminal 2: Frontend (new terminal)
make run-frontend

# Terminal 3: Watch for changes
cd api && go test -v ./...
```

### **Before Committing Code**

```bash
# Format code
make fmt

# Run tests
make test

# Check code quality
make lint

# Clean up
make clean
```

### **Building for Production**

```bash
# Build backend
make build

# Build frontend
make build-frontend

# Results:
# - Backend: ./nofx (run with: ./nofx)
# - Frontend: web/dist/ (served by backend)
```

### **Updating Dependencies**

```bash
# Backend dependencies
make deps-update

# Frontend dependencies
make deps-frontend
```

### **Docker Deployment**

```bash
# Build images
make docker-build

# Start containers
make docker-up

# View logs
make docker-logs

# Stop containers
make docker-down
```

---

## 🌍 Supported Platforms

| Platform | Status | Instructions |
|----------|--------|--------------|
| macOS | ✅ Full Support | `./setup.sh` |
| Ubuntu/Debian | ✅ Full Support | `./setup.sh` |
| CentOS/RHEL | ✅ Full Support | `./setup.sh` |
| Arch Linux | ✅ Full Support | `./setup.sh` |
| Windows 10+ | ✅ Full Support | `setup.bat` |
| WSL2 | ✅ Full Support | `./setup.sh` |
| Docker | ✅ Full Support | `docker-compose up` |

---

## 🤔 Common Questions

### Q: How long does setup take?
**A:** About 5-10 minutes, mostly waiting for downloads.

### Q: Do I need to sign up for all APIs?
**A:** No, only DeepSeek and Binance are required. Others are optional and can be added later.

### Q: Can I test without real money?
**A:** Yes! Use Binance Demo Trading:
- https://demo.binance.com/en/my/settings/api-management
- Copy demo API keys instead of real ones
- Trade with fake money

### Q: How do I update my API keys?
**A:** Run `./configure-keys.sh` anytime to update.

### Q: Is my API key safe?
**A:** Yes!
- Stored in `.env` file (not in git)
- Encrypted at application level
- Never logged or displayed
- Only you have access

### Q: What if I lose my API key?
**A:** Just delete it from your exchange and create a new one:
1. Go to exchange API management
2. Delete/disable old key
3. Create new key
4. Run `./configure-keys.sh` to update

### Q: Can I run this multiple times?
**A:** Yes! It's safe to run `./setup.sh` multiple times. It will ask before overwriting.

### Q: What's in the .env file?
**A:**
- Server ports (8080, 3000)
- Encryption keys (auto-generated, 256-bit security)
- API keys (you provide)
- Timezone settings

### Q: How do I uninstall?
**A:** Just delete the folder. Everything is local, nothing system-wide.

### Q: Can I change ports?
**A:** Yes, edit `.env`:
```
NOFX_BACKEND_PORT=8081
NOFX_FRONTEND_PORT=3001
```

### Q: What if setup hangs?
**A:** Press Ctrl+C to stop. Then:
```bash
./setup.sh
```

It will ask if you want to reconfigure. Say yes.

### Q: Do I need Docker?
**A:** No! It's optional. Setup.sh works without Docker.

### Q: Can I use this on Windows?
**A:** Yes! Use `setup.bat` instead of `./setup.sh`

---

## ✅ Success Checklist

After running `./setup.sh`, verify:

- [ ] No errors in output (warnings are OK)
- [ ] Backend binary created: `./nofx` exists
- [ ] Frontend built: `web/dist/` folder exists
- [ ] `.env` file created with your API keys
- [ ] `make run` starts backend on port 8080
- [ ] `npm start` (in web/) starts frontend on port 3000
- [ ] Browser shows http://localhost:3000

If all checked: ✅ You're ready to trade!

---

## 🚨 When Things Go Wrong

### The nuclear option (resets everything)

```bash
# Stop the app (Ctrl+C in both terminals)

# Clean everything
make clean

# Remove cached dependencies
cd web && rm -rf node_modules package-lock.json
go mod tidy

# Start fresh
./setup.sh
```

### Check system health

```bash
./verify.sh
```

Scroll through output to find ✗ marks (failures).

### Debug mode (see all output)

```bash
# Run setup with debug output
bash -x setup.sh
```

This shows every command being executed.

---

## 📞 Getting Help

If something doesn't work:

1. **Check the Troubleshooting section** (above)
2. **Run `./verify.sh`** to diagnose issues
3. **Check `.env` file** for correct API keys
4. **Look at error messages** carefully
5. **Try the nuclear option** (clean install)

Most issues are:
- Missing Go or Node.js
- Typo in API key
- Port already in use
- Network issue downloading dependencies

All are fixable!

---

## 🎯 What Happens When You Run setup.sh

Here's exactly what the script does:

```
1. Check OS (macOS, Linux, Windows)
2. Verify Go 1.21+ is installed
3. Verify Node.js 18+ is installed
4. Try installing TA-Lib:
   - Homebrew (macOS)
   - apt/yum/pacman (Linux)
   - Source compilation (fallback)
5. Download Go modules
6. Install npm packages (includes franc for language detection)
7. Generate secure encryption keys (JWT, AES-256, RSA)
8. Create .env file from template
9. Open DeepSeek API page in browser
10. Prompt for DeepSeek API key
11. Open Binance website in browser
12. Prompt for Binance API key and secret
13. Save all keys securely to .env
14. Build backend
15. Build frontend (includes franc module)
16. Verify everything works
17. Show startup instructions
```

---

## 🔐 Security Details

### What Gets Generated Automatically

When you run setup, these are created for you:

**JWT_SECRET** (256-bit random)
- Used for API authentication
- Automatically generated
- Random, cryptographically secure
- You never need to touch it

**DATA_ENCRYPTION_KEY** (AES-256)
- Encrypts sensitive data in database
- 32 bytes, Base64 encoded
- Generated once, stored in .env
- You never need to touch it

**RSA_PRIVATE_KEY** (2048-bit)
- Client-server encryption
- Browser-to-server communication
- Generated once, stored in .env
- You never need to touch it

### API Keys

Your personal API keys:
- Stored in `.env` file (git-ignored)
- Never hardcoded in source code
- Encrypted at application level
- Only visible in `.env`
- Can be rotated anytime

---

## 🚀 Next Steps

Once setup completes:

1. **Start the backend:**
   ```bash
   make run
   ```

2. **Start the frontend (new terminal):**
   ```bash
   make run-frontend
   ```

3. **Open browser:**
   ```
   http://localhost:3000
   ```

4. **Configure trading:**
   - Connect your exchange accounts
   - Set trading parameters
   - Review risk settings
   - Start trading!

---

## 📚 Other Scripts (Optional)

These are helpful but not required:

### configure-keys.sh
Update API keys anytime without re-running full setup:
```bash
./configure-keys.sh
```

### verify.sh
Check if everything is installed correctly:
```bash
./verify.sh
```

---

## 🎓 Learning More

- **Backend code:** See `main.go` and `api/` folder
- **Frontend code:** See `web/` folder
- **Configuration:** See `.env.example`
- **Architecture:** See `docs/architecture/README.md`
- **Contributing:** See `CONTRIBUTING.md`

---

## ✨ You're All Set!

That's everything you need to know. Run `./setup.sh` and you're ready to go!

**Questions?** Check the Troubleshooting section above - it covers most issues.

**Happy trading! 🚀**
