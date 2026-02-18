---
title: "CruiseKube Cost Calculation"
description: "How CruiseKube estimates cost and savings. Unit pricing for CPU and memory, assumptions (e.g. c5a.xlarge, on-demand vs spot), calculation formulas, default rates, and where to set Resource Pricing in the dashboard."
keywords:
  - CruiseKube cost calculation
  - cost and savings estimate
  - resource pricing
  - CPU memory unit pricing
  - cost monitoring
  - dashboard pricing configuration
---

CruiseKube provides an estimate of the current cost and savings through its actions. The calculations are done with number of constraints and assumptions. This document is to highlight those. 

Right now the cost monitoring is in beta, and we don't perform in-depth monitoring of resources for cost and saving. We don't keep a time-series record of the resources, which can increase the accuracy. We take a heuristic approach for this, documented below. 

# Unit/Resource Pricing

In the real world, you pay for the instances you have provisioned, and the price depends on whether it's on-demand, spot, or reserved. We take a different approach: we estimate an average price per CPU core/hour and per memory GB/hour, and use these unit prices to calculate cost and savings. 

## Assumptions

- Base instance: `AWS c5a.xlarge` (4 vCPUs, 8 GB RAM).
- Node mix: half on-demand, half spot; no reserved instances.
- Cost split: half the instance price is attributed to CPU, half to memory (network, storage, etc. are treated as included).
- GPU instances are excluded from cost calculation and recommendations.

## Calculation

Based on the pricing from [Instance](https://instances.vantage.sh/aws/ec2/c5a.xlarge?currency=USD)

- Average instance price = ($0.154 + $0.078) ÷ 2 = $0.116/hour.
- From that, price per core/hour = $0.116 ÷ 4 = $0.029/hour, and price per GB/hour = $0.116 ÷ 8 = $0.0145/hour (before the split below).
- The average price (or instance price) is what you pass in the frontend dashboard, and internally we do the below calculation.
- We then assume half of the instance cost is for CPU and half for memory. So the effective rates used in the product are:
    - Price per core/hour = $0.029 ÷ 2 = $0.0145/hour
    - Price per GB/hour = $0.0145 ÷ 2 = $0.00725/hour

## Default prices

| Resource | Default used in calculations |
| --- | --- |
| CPU | **0.0145** $/core/hour |
| Memory | **0.00725** $/GB/hour |

### What prices are used for

All cost and savings figures (current cost, current savings, possible savings etc.) use these hourly rates, converted to monthly amounts using 720 hours per month.

### Where you set it

- CPU and memory prices are configured in **Policies → Resource Pricing**.
- Values are stored in your browser and used only for cost calculations in the dashboard.
