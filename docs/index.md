---
icon: lucide/home
title: CruiseKube
hide:
  - navigation
  - toc
  - footer
---

<div class="ck-hero ck-hero--split">
  <div class="ck-hero__pattern" aria-hidden="true"></div>
  <div class="ck-hero__inner">
    <div class="ck-hero__copy">
      <p class="ck-hero__badge">Kubernetes-native rightsizing · Open source</p>
      <h1 class="ck-hero__title">
        Stop paying for capacity<span class="ck-hero__accent"> your pods never use.</span>
      </h1>
      <p class="ck-hero__lead">
        CruiseKube uses the CPU and memory metrics you already collect to lower <b>request overhead</b> workload by workload—and gives Karpenter room to consolidate nodes. Platform teams <b>cut ~50% of request overhead</b> — without rewriting manifests by hand or flipping a cluster-wide switch.
      </p>
      <div class="ck-hero__actions">
        <a href="/install/gs-installation/" class="ck-btn ck-btn--dark ck-btn-icon">
          <svg class="ck-btn-icon__svg" xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z"/><path d="m12 15-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z"/><path d="M9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0"/><path d="M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5"/></svg>
          Install CruiseKube
        </a>
        <a href="https://truefoundry.github.io/cruiseKube-frontend/" class="ck-btn ck-btn--outline ck-btn-icon" target="_blank" rel="noopener noreferrer">
          <svg class="ck-btn-icon__svg" xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"/><polygon points="10 8 16 12 10 16 10 8"/></svg>
          Try the demo
        </a>
      </div>
      <p class="ck-hero__points" role="list" aria-label="Product highlights">
        <span class="ck-hero__points-item" role="listitem">Installs in 5 minutes</span>
        <span class="ck-hero__points-item" role="listitem">Runs in your cluster</span>
        <span class="ck-hero__points-item" role="listitem">Secure — in-cluster only</span>
      </p>
    </div>

    <div class="ck-hero__figure">
      <img src="/assets/screenshots/demo-overview.png" alt="CruiseKube dashboard and optimization flow">
    </div>
  </div>
</div>

<div class="features-section ck-audiences">
  <div class="section-header ck-audiences__header">
    <p class="ck-eyebrow">Built for both sides of the table</p>
    <h2>DevOps owns the cluster. FinOps owns the bill. CruiseKube gives both a single source of truth.</h2>
  </div>
  <div class="ck-audiences__panel">
    <article class="ck-audiences__col">
      <div class="ck-audiences__meta">
        <span class="ck-audiences__icon" aria-hidden="true">
          <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/></svg>
        </span>
        <p class="ck-audiences__label">For DevOps &amp; Platform</p>
      </div>
      <h3 class="ck-card__title">Stop being the human autoscaler</h3>
      <p class="ck-card__body">
        No more chasing teams to tune requests after every OOM kill. CruiseKube proposes per-workload numbers grounded in Prometheus data — you keep the guardrails, the audit trail, and the kill switch.
      </p>
      <ul class="ck-audiences__list">
        <li class="ck-audiences__item">
          <span class="ck-audiences__check" aria-hidden="true">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
          </span>
          Per-workload Recommend / Cruise modes — promote when ready
        </li>
        <li class="ck-audiences__item">
          <span class="ck-audiences__check" aria-hidden="true">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
          </span>
          Disruption windows so changes never land mid-launch
        </li>
        <li class="ck-audiences__item">
          <span class="ck-audiences__check" aria-hidden="true">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
          </span>
          Critical tiers protect system pods from eviction
        </li>
        <li class="ck-audiences__item">
          <span class="ck-audiences__check" aria-hidden="true">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
          </span>
          Open source — runs in-cluster, no agent calling home
        </li>
      </ul>
    </article>
    <article class="ck-audiences__col">
      <div class="ck-audiences__meta">
        <span class="ck-audiences__icon" aria-hidden="true">
          <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" x2="12" y1="2" y2="22"/><path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg>
        </span>
        <p class="ck-audiences__label">For FinOps &amp; Engineering Leaders</p>
      </div>
      <h3 class="ck-card__title">Savings you can put on a slide</h3>
      <p class="ck-card__body">
        Stop arguing about cluster waste from screenshots. CruiseKube quantifies idle CPU and memory per workload, attributes it to the owning team, and logs every applied change as a structured event.
      </p>
      <ul class="ck-audiences__list">
        <li class="ck-audiences__item">
          <span class="ck-audiences__check" aria-hidden="true">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
          </span>
          Net $/month savings rolled up per workload and namespace
        </li>
        <li class="ck-audiences__item">
          <span class="ck-audiences__check" aria-hidden="true">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
          </span>
          Configurable CPU and memory pricing for accurate cost math
        </li>
        <li class="ck-audiences__item">
          <span class="ck-audiences__check" aria-hidden="true">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
          </span>
          Adoption view: who's on Recommend vs Cruise vs Off
        </li>
        <li class="ck-audiences__item">
          <span class="ck-audiences__check" aria-hidden="true">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
          </span>
          Receipts for finance — every change captured as an event
        </li>
      </ul>
    </article>
  </div>
</div>

<div class="features-section ck-features">
  <div class="section-header">
    <p class="ck-eyebrow">Features</p>
    <h2>From metrics to right-sized requests—and the controls to run it in production</h2>
    <p>Proposed <em>requests</em> from Prometheus, per-workload Recommend and Cruise modes, policies you can stand behind, and full visibility when numbers change.</p>
  </div>
  <div class="ck-connected-grid ck-features__grid">
    <article class="ck-connected-grid__cell ck-features__cell">
      <span class="ck-grid__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 3v18h18"/><path d="M18 17V9"/><path d="M13 17V5"/><path d="M8 17v-3"/></svg>
      </span>
      <p class="ck-eyebrow">Observe</p>
      <h3 class="ck-grid__title">Grounded in Prometheus</h3>
      <p class="ck-grid__body">CPU and memory recommendations from the time series you already scrape—aligned with how each pod actually behaves.</p>
    </article>
    <article class="ck-connected-grid__cell ck-features__cell">
      <span class="ck-grid__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"/><circle cx="12" cy="12" r="3"/></svg>
      </span>
      <p class="ck-eyebrow">Recommend</p>
      <h3 class="ck-grid__title">Review before anything changes</h3>
      <p class="ck-grid__body">Proposals land in the UI. Nothing touches your manifests until you accept them—no blind fleet-wide resize.</p>
    </article>
    <article class="ck-connected-grid__cell ck-features__cell">
      <span class="ck-grid__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22c5.523 0 10-4.477 10-10S17.523 2 12 2 2 6.477 2 12s4.477 10 10 10z"/><path d="m9 12 2 2 4-4"/></svg>
      </span>
      <p class="ck-eyebrow">Cruise</p>
      <h3 class="ck-grid__title">Automate per workload</h3>
      <p class="ck-grid__body">Switch only the workloads you trust to <strong>Cruise</strong>—scoped automation with intent, not a cluster-wide toggle.</p>
    </article>
    <article class="ck-connected-grid__cell ck-features__cell">
      <span class="ck-grid__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z"/><path d="m22 17.65-9.17 4.16a2 2 0 0 1-1.66 0L2 17.67"/><path d="m22 12.65-9.17 4.16a2 2 0 0 1-1.66 0L2 12.67"/></svg>
      </span>
      <p class="ck-eyebrow">Workload-level</p>
      <h3 class="ck-grid__title">Every Deployment gets its own verdict</h3>
      <p class="ck-grid__body">Savings, mode, and filters are scoped to the workloads you run—not a single cluster KPI nobody owns.</p>
    </article>
    <article class="ck-connected-grid__cell ck-features__cell">
      <span class="ck-grid__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/></svg>
      </span>
      <p class="ck-eyebrow">Policies</p>
      <h3 class="ck-grid__title">Guardrails on every change</h3>
      <p class="ck-grid__body"><strong>Policies</strong> cap how aggressive each adjustment can be—enough to cut waste, small enough to trust in production.</p>
    </article>
    <article class="ck-connected-grid__cell ck-features__cell">
      <span class="ck-grid__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/><path d="M16 13H8"/><path d="M16 17H8"/><path d="M10 9H8"/></svg>
      </span>
      <p class="ck-eyebrow">Audit trail</p>
      <h3 class="ck-grid__title">Events that answer “what changed—and when?”</h3>
      <p class="ck-grid__body">Applied recommendations show up as a living log—no reconstructing history from kubectl and Slack threads.</p>
    </article>
    <article class="ck-connected-grid__cell ck-features__cell">
      <span class="ck-grid__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
      </span>
      <p class="ck-eyebrow">Disruption windows</p>
      <h3 class="ck-grid__title">Optimize on <em>your</em> quiet hours</h3>
      <p class="ck-grid__body">Cron-friendly schedules define when it is safe to move numbers—including workloads that cannot shift during peak.</p>
    </article>
    <article class="ck-connected-grid__cell ck-features__cell">
      <span class="ck-grid__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 3v18h18"/><path d="M18 17V9"/><path d="M13 17V5"/><path d="M8 17v-3"/></svg>
      </span>
      <p class="ck-eyebrow">Monitoring</p>
      <h3 class="ck-grid__title">One dashboard for adoption and waste</h3>
      <p class="ck-grid__body">Roll-up views tie cost signals, efficiency, and who is on Recommend vs Cruise—so platform reviews start aligned.</p>
    </article>
    <article class="ck-connected-grid__cell ck-features__cell">
      <span class="ck-grid__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 17 13.5 8.5 8.5 13.5 2 7"/><polyline points="16 17 22 17 22 11"/></svg>
      </span>
      <p class="ck-eyebrow">Outcome</p>
      <h3 class="ck-grid__title">Less unused reservation</h3>
      <p class="ck-grid__body">Lower <em>request</em> overhead and fewer nodes held for headroom you rarely use—without sacrificing observability or control.</p>
    </article>
  </div>
</div>

<div class="features-section ck-stats">
  <div class="ck-stats__panel">
    <div class="ck-band-inset">
    <div class="ck-stats__grid">
      <div class="ck-stats__col">
        <p class="ck-stats__value">&lt; 5 min</p>
        <h3 class="ck-stats__heading">from helm install to insight</h3>
        <p class="ck-stats__desc">Connect your Prometheus and see first recommendations in under five minutes.</p>
      </div>
      <div class="ck-stats__col">
        <p class="ck-stats__value">~50%</p>
        <h3 class="ck-stats__heading">less request overhead</h3>
        <p class="ck-stats__desc">Less CPU and memory reserved on production workloads once Cruise is enabled.</p>
      </div>
      <div class="ck-stats__col">
        <p class="ck-stats__value">~90%</p>
        <h3 class="ck-stats__heading">less optimization toil</h3>
        <p class="ck-stats__desc">Automate cluster tuning—skip spreadsheets, rollouts, and manual request chasing.</p>
      </div>

    </div>
    </div>
  </div>
</div>

<div class="features-section ck-get-started">
  <div class="ck-get-started__card ck-card">
    <div class="ck-get-started__copy">
      <p class="ck-eyebrow">Get started</p>
      <h3 class="ck-card__title ck-get-started__title">One Helm command. First recommendation before you finish standup.</h3>
      <ul class="ck-get-started__list">
        <li class="ck-get-started__item">
          <span class="ck-get-started__check" aria-hidden="true">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
          </span>
          Needs only Prometheus — no vendor lock-in
        </li>
        <li class="ck-get-started__item">
          <span class="ck-get-started__check" aria-hidden="true">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
          </span>
          First recommendations in under 5 minutes
        </li>
        <li class="ck-get-started__item">
          <span class="ck-get-started__check" aria-hidden="true">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
          </span>
          Open source — runs entirely in your cluster
        </li>
      </ul>
      <div class="ck-get-started__actions">
        <a href="/install/gs-installation/" class="ck-btn ck-btn--dark">Install CruiseKube</a>
        <a href="/about/overview/" class="ck-btn ck-btn--outline">Read the docs</a>
      </div>
    </div>
    <div class="ck-get-started__terminal" aria-label="Example Helm install command">
      <div class="ck-terminal__bar">
        <span class="ck-terminal__dot ck-terminal__dot--red" aria-hidden="true"></span>
        <span class="ck-terminal__dot ck-terminal__dot--yellow" aria-hidden="true"></span>
        <span class="ck-terminal__dot ck-terminal__dot--green" aria-hidden="true"></span>
        <span class="ck-terminal__label">terminal</span>
      </div>
      <pre class="ck-terminal__body"><code><span class="ck-tok-cmd">helm install</span> cruisekube \
  oci://tfy.jfrog.io/tfy-helm/cruisekube \
  <span class="ck-tok-flag">--namespace</span> cruisekube-system \
  <span class="ck-tok-flag">--create-namespace</span> \
  <span class="ck-tok-flag">--set</span> cruisekubeController.env.\
CRUISEKUBE_DEPENDENCIES_INCLUSTER_PROMETHEUSURL=\
<span class="ck-tok-str">"http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090"</span>

<span class="ck-tok-ok">✓ Release "cruisekube" installed successfully</span></code></pre>
    </div>
  </div>
</div>

<div class="features-section ck-community">
  <div class="section-header ck-community__header">
    <p class="ck-eyebrow">Get involved</p>
    <h2>Questions, contributions, and honest bug reports — all in the open.</h2>
  </div>
  <nav class="ck-community__grid" aria-label="Community links">
    <a class="ck-community__card" href="https://github.com/truefoundry/CruiseKube" rel="noopener noreferrer">
      <span class="ck-community__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="18" r="3"/><circle cx="6" cy="6" r="3"/><circle cx="18" cy="6" r="3"/><path d="M18 9v2c0 .6-.4 1-1 1H7c-.6 0-1-.4-1-1V9"/><path d="M12 12v3"/></svg>
      </span>
      <h3 class="ck-community__title">Star us on GitHub</h3>
      <p class="ck-community__meta">Browse the source, file issues, send PRs. CruiseKube is open source — read it, run it in your cluster, contribute back.</p>
      <span class="ck-community__link">github.com/truefoundry/cruisekube <span aria-hidden="true">→</span></span>
    </a>
    <a class="ck-community__card" href="/about/overview/" rel="noopener">
      <span class="ck-community__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 7v14"/><path d="M3 18a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h5a4 4 0 0 1 4 4 4 4 0 0 1 4-4h5a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1h-6a3 3 0 0 0-3 3 3 3 0 0 0-3-3z"/></svg>
      </span>
      <h3 class="ck-community__title">Read the docs</h3>
      <p class="ck-community__meta">Install guide, architecture overview, configuration reference, and the Prometheus queries CruiseKube relies on.</p>
      <span class="ck-community__link">Open docs <span aria-hidden="true">→</span></span>
    </a>
    <a class="ck-community__card" href="https://calendar.app.google/aJGRuUjSjNd15vqJ9" target="_blank" rel="noopener noreferrer">
      <span class="ck-community__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M7.9 20A9 9 0 1 0 4 16.1L2 22Z"/></svg>
      </span>
      <h3 class="ck-community__title">Talk to a human</h3>
      <p class="ck-community__meta">Want help with a first pilot, or have a workload pattern CruiseKube doesn't handle yet? Book a short call.</p>
      <span class="ck-community__link">Schedule a call <span aria-hidden="true">→</span></span>
    </a>
  </nav>
</div>

<div class="features-section ck-cta">
  <div class="ck-cta__panel">
    <div class="ck-band-inset">
      <div class="ck-cta__inner">
        <h2 class="ck-cta__title">Cut cluster cost — on your timeline.</h2>
        <p class="ck-cta__lead">
          Install when you're ready, or book a short call if you want help with a first pilot. CruiseKube runs entirely inside your cluster — your metrics, your guardrails, your call.
        </p>
        <div class="ck-cta__actions">
          <a href="/install/gs-installation/" class="ck-btn ck-btn--light">Start saving <span aria-hidden="true">→</span></a>
          <a href="https://calendar.app.google/aJGRuUjSjNd15vqJ9" class="ck-btn ck-btn--ghost" target="_blank" rel="noopener noreferrer">Schedule a call</a>
        </div>
      </div>
    </div>
  </div>
</div>
