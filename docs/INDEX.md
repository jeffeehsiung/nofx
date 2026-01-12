# 📑 Documentation Quick Index

> **Start Here:** [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) - Comprehensive guide to all systems (consolidated from 6 docs into 1)

---

## 🎯 What You Need Right Now

| Your Question | Section in DEVELOPER_ONBOARDING.md |
|---------------|-----------------------------------|
| **"Where do I start?"** | Section 1: Quick Start |
| **"How does the system work?"** | Section 2: Architecture |
| **"How do I control everything?"** | Section 7: Frontend Control Guide |
| **"Are all 6 systems integrated?"** | Section 8: System Integration Verification |
| **"What was fixed recently?"** | Section 9: Critical Fixes |
| **"How do I calibrate?"** | Section 10: Calibration Operations |
| **"What needs to be verified?"** | Section 11: Verification Checklist |
| **"Are there unused functions?"** | Section 12: Code Audit |

---

## 📚 Main Resources

### 🚀 Getting Started (Choose Your Path)

| Document | For Who | Content |
|----------|---------|---------|
| [README.md](../README.md) | Everyone | Project overview, quick start |
| [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) | **Developers** | **Complete consolidated guide - READ THIS FIRST** |
| [getting-started/docker-deploy.en.md](getting-started/docker-deploy.en.md) | Quick deployment | Docker setup instructions |

### 🏗️ Architecture & System Design

All architecture details are covered in [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) Sections 2-10.

**Additional Resources:**
- [architecture/README.md](architecture/README.md) - System modules
- [architecture/STRATEGY_MODULE.md](architecture/STRATEGY_MODULE.md) - Strategy configuration
- [architecture/BACKTEST_MODULE.md](architecture/BACKTEST_MODULE.md) - Backtesting system
- [architecture/DEBATE_MODULE.md](architecture/DEBATE_MODULE.md) - Multi-AI debate

### 📖 Additional Resources

**Trade Failure Analysis & Calibration:**
- [threshold-calibration.md](threshold-calibration.md) - Complete calibration guide
- [magic-number-elimination-summary.md](magic-number-elimination-summary.md) - Implementation summary

**AI Prompt Engineering:**
- [prompt-guide.md](prompt-guide.md) - How to optimize AI prompts

**Financial Concepts:**
- [pnl.md](pnl.md) - P&L calculation and position tracking

---

## 🎓 Learning Paths

### Path 1: Quick Start (User)
```
1. README.md (10 min)
2. getting-started/docker-deploy.en.md (15 min)
3. Deploy and start trading!
```

### Path 2: Developer Onboarding (Complete)
```
1. README.md (10 min)
2. DEVELOPER_ONBOARDING.md - Day 1 (3 hours)
   ├─ Section 1: Quick Start
   ├─ Section 2: Architecture
   └─ Section 3: Core Components

3. DEVELOPER_ONBOARDING.md - Day 2 (3 hours)
   ├─ Section 4: Trade Failure Analysis
   ├─ threshold-calibration.md (detailed reference)
   └─ Run calibration tests

4. DEVELOPER_ONBOARDING.md - Day 3 (3 hours)
   ├─ Section 5: Backtest Analysis
   └─ Run and analyze your first backtest

5. DEVELOPER_ONBOARDING.md - Day 4 (3 hours)
   ├─ Section 7: Verification
   ├─ Section 8: Code Audit
   └─ Section 9: Integration Check

6. DEVELOPER_ONBOARDING.md - Day 5 (3 hours)
   └─ Complete graduation exercise (Section 10)
```

---

## 🎓 Learning Paths

All learning paths are in [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md).

---

## 🔍 Quick File Finder

### Critical Files
- [main.go](../main.go) - Entry point (lines 1-170)
- [trader/trader.go](../trader/trader.go) - Trading loop (lines 1-300)
- [decision/engine.go](../decision/engine.go) - AI decisions (lines 1-500)
- [market/data.go](../market/data.go) - Market data (lines 1-200)

### New Features (Jan 2026)
- `decision/threshold_calibrator.go` - Data-driven thresholds
- `decision/trade_failure.go` - Failure analysis  
- `market/microstructure.go` - Order book analysis
- `backtest/calibration_scheduler.go` - Monthly auto-calibration
- `store/trade_outcome.go` - Live trade recording
- `web/src/pages/PromptLabPage.tsx` - Prompt evolution UI

---

## 📊 Documentation Status

✅ **Complete & Current:**
- [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) - Master guide (1500+ lines, 13 sections)
- [threshold-calibration.md](threshold-calibration.md) - Calibration reference
- [magic-number-elimination-summary.md](magic-number-elimination-summary.md) - Implementation details
- [architecture/](architecture/) - System architecture docs
- [README.md](../README.md) - Project overview

---

## 🎯 Your Next Steps

**New developer?** → Follow [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) (15 hours)  
**Need specific info?** → Use "What You Need Right Now" table at top  
**Stuck?** → Check [DEVELOPER_ONBOARDING.md - Common Issues](DEVELOPER_ONBOARDING.md)

---

**Last Updated:** January 12, 2026 | **Status:** All systems integrated ✅
