package handlers

import "time"

// StatsAPIDataLookbackWindow is the time window for /stats, /workloads and /recommendation-analysis APIs.
// Only data updated within this window is returned.
const StatsAPIDataLookbackWindow = 24 * time.Hour
