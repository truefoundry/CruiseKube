---
icon: lucide/home
title: CruiseKube
hide:
  - navigation
  - toc
---

<div class="ck-hero">
  <div class="ck-hero__inner">
    <div class="ck-hero__copy">
      <p class="ck-hero__eyebrow">Kubernetes-native · Open source</p>
      <h1 class="ck-hero__title">
        Cut cluster spend in half
      </h1>
      <p class="ck-hero__lead">
        Right-size CPU and memory from real usage. Start in Recommend, enable Cruise when you trust it, often <strong>~50%</strong> less request overhead, without a tuning spreadsheet.
      </p>
      <div class="ck-hero__actions">
        <a href="/install/gs-installation/" class="md-button md-button--primary ck-btn-icon">
          <svg class="ck-btn-icon__svg" xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z"/><path d="m12 15-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z"/><path d="M9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0"/><path d="M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5"/></svg>
          Start Saving
        </a>
        <a href="/about/overview/" class="md-button">How it works</a>
      </div>
    </div>
    <figure class="ck-hero__figure">
      <img src="/assets/images/hero.png" alt="CruiseKube dashboard and optimization flow">
    </figure>
  </div>
</div>

<!-- Why optimization is hard today -->
<div class="features-section">
  <div class="section-header">
    <h2>Why clusters still leak</h2>
    <p>Right-sizing is a familiar idea with a fragile day-to-day workflow.</p>
  </div>
  <div class="ck-problems-copy ck-points">
    <ul class="ck-points__list">
      <li class="ck-points__item">
        <span class="ck-points__icon" aria-hidden="true">
          <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z"/><path d="m22 17.65-9.17 4.16a2 2 0 0 1-1.66 0L2 17.67"/><path d="m22 12.65-9.17 4.16a2 2 0 0 1-1.66 0L2 12.67"/></svg>
        </span>
        <span class="ck-points__text">CPU and memory <em>requests</em> are often chosen once—from a template, a spike, or caution—and then left alone. Schedulers and bills follow those numbers, so a little extra headroom on every pod becomes a lot of reservation you never use.</span>
      </li>
      <li class="ck-points__item">
        <span class="ck-points__icon" aria-hidden="true">
          <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="7" height="9" x="3" y="3" rx="1"/><rect width="7" height="5" x="14" y="3" rx="1"/><rect width="7" height="9" x="14" y="12" rx="1"/><rect width="7" height="5" x="3" y="16" rx="1"/></svg>
        </span>
        <span class="ck-points__text">Digging through dashboards for every workload does not keep up with how fast teams ship.</span>
      </li>
      <li class="ck-points__item">
        <span class="ck-points__icon" aria-hidden="true">
          <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10"/><path d="M12 8v4"/><path d="M12 16h.01"/></svg>
        </span>
        <span class="ck-points__text">Flipping a switch and resizing everything without guardrails does not pass a change review. Most teams sit in the middle: they know the numbers are wrong, and they still do not move them.</span>
      </li>
    </ul>
  </div>
</div>

<!-- Why CruiseKube -->
<div class="features-section">
  <div class="section-header">
    <h2>How CruiseKube answers that</h2>
    <p>Suggestions grounded in how your pods actually run—and automation only where you turn it on.</p>
  </div>
  <div class="ck-problems-copy ck-points">
    <ul class="ck-points__list">
      <li class="ck-points__item">
        <span class="ck-points__icon" aria-hidden="true">
          <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 3v18h18"/><path d="M18 17V9"/><path d="M13 17V5"/><path d="M8 17v-3"/></svg>
        </span>
        <span class="ck-points__text">CruiseKube uses the CPU and memory signals you already collect in Prometheus and turns them into concrete <em>request</em> values. You align reservations with observed behavior—not a template or a one-off dashboard export.</span>
      </li>
      <li class="ck-points__item">
        <span class="ck-points__icon" aria-hidden="true">
          <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"/><circle cx="12" cy="12" r="3"/></svg>
        </span>
        <span class="ck-points__text"><strong>Recommend first.</strong> See proposed requests in the UI, compare them to what you run today, and adopt what makes sense. When you are ready, enable Cruise for specific workloads so updates roll out in a controlled way—not as a cluster-wide blind switch.</span>
      </li>
      <li class="ck-points__item">
        <span class="ck-points__icon" aria-hidden="true">
          <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 17 13.5 8.5 8.5 13.5 2 7"/><polyline points="16 17 22 17 22 11"/></svg>
        </span>
        <span class="ck-points__text"><strong>Spend less on unused reservation.</strong> Many teams see a large drop in request overhead while policies and safeguards keep the blast radius of each change understandable—less waste without treating stability as an afterthought.</span>
      </li>
    </ul>
  </div>
</div>

<!-- Product features: workloads, guardrails, audit, windows, monitoring -->
<div class="features-section ck-showcase">
  <div class="section-header">
    <h2>Features</h2>
    <p>Workload-shaped recommendations, guardrails you can stand behind, and an audit trail when requests move.</p>
  </div>

  <div class="ck-showcase__list">

    <article class="ck-showcase__row">
      <div class="ck-showcase__copy">
        <p class="ck-showcase__label">Workload-level</p>
        <h3 class="ck-showcase__title">Every Deployment gets its own verdict</h3>
        <p class="ck-showcase__body">
          Savings, mode, and filters are scoped to the workloads you run—not a single cluster KPI nobody owns. Open a service, compare <em>today’s</em> requests to what the data says, and decide what earns the next step.
        </p>
      </div>
      <figure class="ck-showcase__figure">
        <img src="/assets/screenshots/feat_workload.png" alt="Workload detail: per-pod CPU and memory recommendations vs current requests">
      </figure>
    </article>

    <article class="ck-showcase__row ck-showcase__row--flip">
      <div class="ck-showcase__copy">
        <p class="ck-showcase__label">Guardrails</p>
        <h3 class="ck-showcase__title">Recommend-only until <em>you</em> promote it</h3>
        <p class="ck-showcase__body">
          Start in <strong>Recommend</strong>: proposals land in the UI, nothing rewrites requests behind your back. When a team is ready, switch that workload to <strong>Cruise</strong>, automation with intent, not a blind fleet-wide toggle.
        </p>
      </div>
      <figure class="ck-showcase__figure">
        <img src="/assets/screenshots/feat_recommendation_critical.png" alt="Policies and configuration: CruiseKube mode, priority, and safety settings">
      </figure>
    </article>

    <article class="ck-showcase__row">
      <div class="ck-showcase__copy">
        <p class="ck-showcase__label">Audit trail</p>
        <h3 class="ck-showcase__title">Events that answer “what changed—and when?”</h3>
        <p class="ck-showcase__body">
          Applied recommendations show up as a living log. When finance or SRE asks for receipts, you are not reconstructing history from kubectl and Slack threads.
        </p>
      </div>
      <figure class="ck-showcase__figure">
        <img src="/assets/screenshots/feat_events.png" alt="Events view: audit log of recommendations and configuration changes">
      </figure>
    </article>

    <article class="ck-showcase__row ck-showcase__row--flip">
      <div class="ck-showcase__copy">
        <p class="ck-showcase__label">Disruption windows</p>
        <h3 class="ck-showcase__title">Optimize on <em>your</em> quiet hours</h3>
        <p class="ck-showcase__body">
          Define when it is safe to move numbers—cron-friendly schedules and room for the workloads that cannot shift during peak. 
        </p>
      </div>
      <figure class="ck-showcase__figure">
        <img src="/assets/screenshots/feat_disruption.png" alt="Disruption window builder with schedule and UTC summary">
      </figure>
    </article>

    <article class="ck-showcase__row">
      <div class="ck-showcase__copy">
        <p class="ck-showcase__label">Monitoring</p>
        <h3 class="ck-showcase__title">One dashboard for adoption, waste, and “are we winning?”</h3>
        <p class="ck-showcase__body">
          Roll-up views tie cost signals, resource efficiency, and who is still on Recommend vs Cruise—so platform reviews start with a shared picture.
        </p>
      </div>
      <figure class="ck-showcase__figure">
        <img src="/assets/screenshots/feat_overview.png" alt="Dashboard overview: cost, adoption, and cluster efficiency">
      </figure>
    </article>

  </div>
</div>


<div class="features-section ck-community">
  <div class="section-header ck-community__header">
    <h2>Get Involved</h2>
    <p>Questions, contributions, and honest bug reports all land in the open.</p>
  </div>
  <nav class="ck-community__grid" aria-label="Community links">
    <a class="ck-community__card" href="https://github.com/truefoundry/CruiseKube" rel="noopener noreferrer">
      <span class="ck-community__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.403 5.403 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4"/><path d="M9 18c-4.51 2-5-2-7-2"/></svg>
      </span>
      <span class="ck-community__title">GitHub</span>
      <span class="ck-community__meta">Source, releases, and pull requests</span>
    </a>
    <a class="ck-community__card" href="https://discord.gg/Dqek4xJa3N" rel="noopener noreferrer">
      <span class="ck-community__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 9a2 2 0 0 1-2 2H6l-4 4V4a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2z"/><path d="M18 9h2a2 2 0 0 1 2 2v11l-4-4h-6a2 2 0 0 1-2-2v-1"/></svg>
      </span>
      <span class="ck-community__title">Discord</span>
      <span class="ck-community__meta">Chat with maintainers and other operators</span>
    </a>
    <a class="ck-community__card" href="https://github.com/truefoundry/CruiseKube/issues" rel="noopener noreferrer">
      <span class="ck-community__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m8 2 1.88 1.88"/><path d="M14.12 3.88 16 2"/><path d="M9 7.13v-1a3.003 3.003 0 1 1 6 0v1"/><path d="M12 20c-3.3 0-6-2.7-6-6v-3a4 4 0 0 1 4-4h4a4 4 0 0 1 4 4v3c0 3.3-2.7 6-6 6"/><path d="M12 20v-9"/><path d="M6.53 9C4.6 8.75 3 6.77 3 4.5 3 2.6 5.02 1 7.5 1c1.22 0 2.4.45 3.3 1.2"/><path d="M17.47 9c1.93-.25 3.53-2.23 3.53-4.5C21 2.6 18.98 1 16.5 1c-1.22 0-2.4.45-3.3 1.2"/></svg>
      </span>
      <span class="ck-community__title">Issues</span>
      <span class="ck-community__meta">Report bugs or request features</span>
    </a>
  </nav>
</div>

<div class="features-section ck-cta">
  <div class="ck-cta__panel">
    <div class="section-header ck-cta__header">
      <h2>Cut cluster cost—on your timeline</h2>
      <p>
        <strong>Install</strong> when you are ready, or <strong>book a short call</strong> if you want help with a first pilot.
      </p>
    </div>
    <div class="ck-cta__actions">
      <a href="/install/gs-installation/" class="md-button md-button--primary ck-btn-icon">
        <svg class="ck-btn-icon__svg" xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z"/><path d="m12 15-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z"/><path d="M9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0"/><path d="M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5"/></svg>
        Start Saving
      </a>
      <a href="https://calendar.app.google/aJGRuUjSjNd15vqJ9" class="md-button ck-btn-icon" target="_blank" rel="noopener noreferrer">
        <svg class="ck-btn-icon__svg" xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M8 2v4"/><path d="M16 2v4"/><rect width="18" height="18" x="3" y="4" rx="2"/><path d="M3 10h18"/></svg>
        Schedule a call
      </a>
    </div>
  </div>
</div>
