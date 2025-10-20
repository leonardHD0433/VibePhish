# FYPhish Quick Start - For Teammates 🚀

Get FYPhish running in **3 commands, 2 minutes**.

## What You Need

- ✅ Docker installed
- ✅ Go 1.24+ installed
- ✅ This repository cloned

## Setup Commands

```bash
# 1. Clone repository
git clone <your-repo-url>
cd FYPhish

# 2. (Optional) Configure environment
cp .env.example .env
# Edit .env if you want Microsoft SSO

# 3. Run everything
./run.sh
```

## Done! 🎉

Access FYPhish at: **http://localhost:3333**

Default login:
- Username: `admin`
- Password: `gophish`

## Useful Commands

```bash
./run.sh              # Start (default)
./run.sh --rebuild    # Rebuild containers
./run.sh --stop       # Stop containers
./run.sh --clean      # Remove everything
```

## What's Running?

- **PostgreSQL**: `localhost:5432` (background)
- **FYPhish Admin**: `localhost:3333` (foreground)
- **Phishing Server**: `localhost:8080` (foreground)

## Database Credentials

```
postgres://fyphish_user:fyphish_dev_2025@localhost:5432/fyphish
```

## Troubleshooting

**Port 5432 busy?**
```bash
sudo systemctl stop postgresql  # Stop system PostgreSQL
```

**Container errors?**
```bash
./run.sh --clean
./run.sh --rebuild
```

**Need help?**
```bash
./run.sh --help
```

---

📖 **Full setup guide**: [SETUP.md](SETUP.md)
