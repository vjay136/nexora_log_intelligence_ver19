# NLI Ver19 support matrix

Updated: 2026-10-21

This matrix separates locally measured results from release gates that require
Linux containers or real Rocky/RHEL hosts.

| Area | Intended support | Current Ver19 evidence |
|---|---|---|
| Upgrade source | Released Ver18 `8a084b8` | Source baseline identified; in-place package lifecycle awaits RPM gate |
| Client/controller OS | x86_64 RHEL/Rocky 8, 9, 10 | Go and installer contracts pass; container/Rocky lifecycle blocked locally because Docker is unavailable |
| Controller access | private TLS, corporate certificate, Nginx/ACME, existing HTTPS proxy, Cloudflare Tunnel | Static and contract checks pass; deployment remains environment-specific |
| Hosted WebApp | Vercel and Cloudflare Pages through same-origin proxy | Vite build and 20 deployment/proxy contracts pass |
| Browser | current Chromium-class browsers | Playwright Chromium suite passes 57/57 for Release 6 |
| Parser evolution | NUPIRE discovery, declarative compilation, held-out evaluation, shadow, approval, canary, promotion, drift, rollback | Unit, API, persistence, UI, and fuzz checks pass |
| Production rollout | existing signed, independently approved parser rollout | Preserved; NUPIRE promotion cannot bypass it |
| Supabase | protected metadata and explicitly retained encrypted artifacts | Existing Ver18 contract retained; secrets remain controller-only |
| Arbitrary unknown logs | lossless quarantine and explicit partial/abstain outcomes | Supported safety behavior; universal semantic accuracy is not claimed |

Release status is recorded in [V19_ACCEPTANCE_CHECKLIST.md](V19_ACCEPTANCE_CHECKLIST.md).
