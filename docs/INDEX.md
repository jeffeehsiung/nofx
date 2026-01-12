# 📑 Documentation Quick Index

> **New Developer?** Start with [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) - your complete guide!

---

## 🎯 What You Need Right Now

| Your Question | Read This Document | Section |
|---------------|-------------------|---------|
| **"Where do I start?"** | [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) | Section 1: Quick Start |
| **"How does the system work?"** | [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) | Section 2: Architecture |
| **"Is my feedback loop working?"** | [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) | Section 4: Trade Failure Analysis |
| **"Why did my backtest fail?"** | [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) | Section 5: Backtest Analysis |
| **"How do I optimize?"** | [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) | Section 5.4: Optimization Workflow |
| **"What is microstructure?"** | [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) | Section 6: Market Microstructure |
| **"Are all features used by default?"** | [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) | Section 9: Integration Verification |
| **"Any unused code?"** | [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) | Section 8: Code Audit |

---

## 📚 Complete Documentation Map

### 🚀 Getting Started (Choose Your Path)

| Document | For Who | Time |
|----------|---------|------|
| [README.md](../README.md) | Everyone - Start here | 10 min |
| [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) | **Developers - Complete guide** | 2-3 hours |
| [getting-started/docker-deploy.en.md](getting-started/docker-deploy.en.md) | Quick deployment | 15 min |

---

### 🏗️ Architecture & System Design

| Document | Purpose | What You'll Learn |
|----------|---------|-------------------|
| [architecture/README.md](architecture/README.md) | System overview | Components, data flow, modules |
| [architecture/STRATEGY_MODULE.md](architecture/STRATEGY_MODULE.md) | Strategy system | How AI strategies work |
| [architecture/BACKTEST_MODULE.md](architecture/BACKTEST_MODULE.md) | Backtesting | How to test strategies |
| [architecture/DEBATE_MODULE.md](architecture/DEBATE_MODULE.md) | Multi-AI debate | 5 AI agents collaborating |

**🎯 Start with:** [DEVELOPER_ONBOARDING.md - Section 2](DEVELOPER_ONBOARDING.md#2-system-architecture-overview)

---

### 🔧 Core Systems Deep Dive

#### Trade Decision System
| Document | Focus | Key Files |
|----------|-------|-----------|
| [DEVELOPER_ONBOARDING.md - Section 3.3](DEVELOPER_ONBOARDING.md#33-decision-engine) | AI decision making | `decision/engine.go`, `decision/prompt_builder.go` |
| [prompt-guide.md](prompt-guide.md) | Prompt engineering | How to optimize AI prompts |
| [architecture/STRATEGY_MODULE.md](architecture/STRATEGY_MODULE.md) | Strategy config | How strategies are built |

#### Trade Failure Analysis (NEW! 🔥)
| Document | Focus | Key Files |
|----------|-------|-----------|
| [threshold-calibration.md](threshold-calibration.md) | **Complete calibration guide** | `decision/threshold_calibrator.go` |
| [magic-number-elimination-summary.md](magic-number-elimination-summary.md) | Implementation summary | `decision/trade_failure.go` |
| [DEVELOPER_ONBOARDING.md - Section 4](DEVELOPER_ONBOARDING.md#4-trade-failure-analysis--feedback-loop) | Integration & usage | How to use in your system |

**🎯 Key Innovation:** Data-driven thresholds replace magic numbers using ROC analysis + Youden's J statistic

#### Market Data & Microstructure
| Document | Focus | Key Files |
|----------|-------|-----------|
| [DEVELOPER_ONBOARDING.md - Section 6](DEVELOPER_ONBOARDING.md#6-market-microstructure-system) | Order book analysis | `market/microstructure.go` |
| [DEVELOPER_ONBOARDING.md - Section 3.4](DEVELOPER_ONBOARDING.md#34-market-data-system) | Data pipeline | `market/data.go`, `market/*_websocket.go` |

#### Backtesting & Optimization
| Document | Focus | Key Files |
|----------|-------|-----------|
| [architecture/BACKTEST_MODULE.md](architecture/BACKTEST_MODULE.md) | Backtest system | `backtest/runner.go`, `backtest/manager.go` |
| [DEVELOPER_ONBOARDING.md - Section 5](DEVELOPER_ONBOARDING.md#5-backtest-analysis--optimization) | **Analysis & optimization** | How to improve strategies |
| [pnl.md](pnl.md) | P&L calculation | Position tracking |

---

### ✅ Verification & Quality

| Document | Purpose | When to Use |
|----------|---------|-------------|
| [DEVELOPER_ONBOARDING.md - Section 7](DEVELOPER_ONBOARDING.md#7-verification-checklist) | System verification | After integration, before production |
| [DEVELOPER_ONBOARDING.md - Section 8](DEVELOPER_ONBOARDING.md#8-code-audit--unused-functions) | Code audit | Find unused code, cleanup |
| [DEVELOPER_ONBOARDING.md - Section 9](DEVELOPER_ONBOARDING.md#9-integration--usage-verification) | Integration check | Ensure features work by default |

---

### 📖 User & API Guides

| Document | For Who | Content |
|----------|---------|---------|
| [guides/README.md](guides/README.md) | End users | Usage guides |
| [guides/faq.en.md](guides/faq.en.md) | Troubleshooting | Common issues |
| [api/](api/) | API developers | REST API reference |

---

### 🛠️ Development & Contributing

| Document | Purpose |
|----------|---------|
| [CONTRIBUTING.md](../CONTRIBUTING.md) | How to contribute |
| [MIGRATION_GUIDE.md](MIGRATION_GUIDE.md) | Version upgrades |
| [Git工作流规范.md](Git工作流规范.md) | Git workflow (Chinese) |
| [CHANGES_REFERENCE.md](CHANGES_REFERENCE.md) | Recent changes |

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

### Path 3: Specific Topics

**"I want to understand AI decisions"**
```
1. DEVELOPER_ONBOARDING.md - Section 3.3
2. architecture/STRATEGY_MODULE.md
3. prompt-guide.md
4. Read: decision/engine.go, decision/prompt_builder.go
```

**"I want to improve backtest results"**
```
1. DEVELOPER_ONBOARDING.md - Section 5
2. architecture/BACKTEST_MODULE.md
3. threshold-calibration.md
4. Run A/B tests with run_ab_tests.sh
```

**"I want to optimize market analysis"**
```
1. DEVELOPER_ONBOARDING.md - Section 6
2. Read: market/microstructure.go
3. Read: market/order_book_monitor.go
4. Implement calibration for remaining magic numbers
```

**"I want to add new features"**
```
1. DEVELOPER_ONBOARDING.md - Section 8 (understand codebase)
2. CONTRIBUTING.md (contribution guidelines)
3. Create feature, test, submit PR
```

---

## 🔍 Quick File Finder

### Critical Files (Read These First)

| File | Purpose | Lines to Focus |
|------|---------|----------------|
| [main.go](../main.go) | Application entry point | 1-170 (startup sequence) |
| [trader/trader.go](../trader/trader.go) | Trading loop | 1-300 (core logic) |
| [decision/engine.go](../decision/engine.go) | AI decision making | 1-500 (decision flow) |
| [market/data.go](../market/data.go) | Market data | 1-200 (data aggregation) |

### New Features (Our Work)

| File | Purpose | Read First |
|------|---------|------------|
| [decision/threshold_calibrator.go](../decision/threshold_calibrator.go) | Data-driven thresholds | Lines 1-150 |
| [decision/trade_failure.go](../decision/trade_failure.go) | Failure analysis | Lines 60-300 |
| [market/microstructure.go](../market/microstructure.go) | Order book analysis | Lines 1-100, 200-300 |

### Integration Points (Check These)

| File | Check For | Should Find |
|------|-----------|-------------|
| [trader/trader.go](../trader/trader.go) | `AnalyzeFailedTrade` call | In `onPositionClose()` |
| [backtest/runner.go](../backtest/runner.go) | `AnalyzeFailedTrade` call | After trade close |
| [manager/trader_manager.go](../manager/trader_manager.go) | `CalibrateFromHistory` | In scheduler |

---

## 🚨 Common Issues Quick Lookup

| Issue | Document | Section |
|-------|----------|---------|
| Build errors | [guides/faq.en.md](guides/faq.en.md) | Troubleshooting |
| Deployment issues | [getting-started/docker-deploy.en.md](getting-started/docker-deploy.en.md) | FAQ section |
| Trade failure analysis not working | [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) | Section 9.1 |
| Calibration "insufficient data" | [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) | Common Issues |
| Microstructure not affecting decisions | [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) | Common Issues |
| Unused functions found | [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) | Section 8.2 |

---

## 📊 Documentation Status

### ✅ Complete & Up-to-Date
- [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) - Comprehensive guide (NEW)
- [threshold-calibration.md](threshold-calibration.md) - Complete calibration reference (NEW)
- [magic-number-elimination-summary.md](magic-number-elimination-summary.md) - Implementation summary (NEW)
- [architecture/README.md](architecture/README.md) - System architecture
- [README.md](../README.md) - Project overview

### ⚠️ May Need Updates
- [architecture/BACKTEST_MODULE.md](architecture/BACKTEST_MODULE.md) - Add failure analysis integration
- [architecture/STRATEGY_MODULE.md](architecture/STRATEGY_MODULE.md) - Add threshold configuration

### 🚧 Planned Documentation
- Advanced optimization guide
- Custom indicator development
- Production deployment best practices
- Performance tuning guide

---

## 🎯 Your Next Steps

**If you're a new developer:**
1. Read [README.md](../README.md) (10 minutes)
2. **Follow [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md) Day 1-5** (15 hours total)
3. Complete the graduation exercise
4. You're ready to contribute! 🎉

**If you need specific information:**
1. Check the **"What You Need Right Now"** table at the top
2. Jump directly to the relevant section

**If you're stuck:**
1. Check [DEVELOPER_ONBOARDING.md - Common Issues](DEVELOPER_ONBOARDING.md#-common-issues--solutions)
2. Search this file for keywords
3. Check [guides/faq.en.md](guides/faq.en.md)

---

## 📝 Documentation Feedback

Found an issue? Have suggestions?
- Open an issue on GitHub
- PR welcome for doc improvements
- See [CONTRIBUTING.md](../CONTRIBUTING.md)

---

**Last Updated:** January 12, 2026  
**Key Contributors:** Development team + AI optimization work  
**Status:** Active development - docs updated continuously
