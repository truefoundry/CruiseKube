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

CruiseKube's provides an estimate on the current cost and saving through CruiseKube. The calculations are right now done with number of constraints and assumptions. This document is to highlight those. 

Right now the cost monitoring is in beta, and we don't perform in-depth monitoring of resources for cost and saving. We don't keep a time-series record of the resources, which can increase the accuracy. We take a heuristic approach for this, documented below. 

# Unit/Resource Pricing

In real world, you pay for the instances/node you have provisioned, and it depends if its on demand, spot, reserved etc. We take a different approach, we estimate the average price for CPU: core/hour and per Memory: GB/hour, and use this unit pricing to calculate the cost and saving. 

### Assumptions

- We use `AWS c5a.xlarge`: 4 vCPUs, 8 GB memory as base instance.
- We assume that half of the nodes are on-demand, and half spot, with no reserved.
- We assume, half of the price is for CPU, and half is for Memory, with all the other factors inclusive in this price, like network, storage etc.
- We exclude GPU instance from calculation and recommendation for this.

### Calculation

Based on the pricing from [Instance](https://instances.vantage.sh/aws/ec2/c5a.xlarge?currency=USD)

- Average instance price = ($0.154 + $0.078) ÷ 2 = $0.116/hour.
- From that, price per core/hour = $0.116 ÷ 4 = $0.029/hour, and price per GB/hour = $0.116 ÷ 8 = $0.0145/hour (before the split below).
- The average price/or instance price is what you pass in the frontend dashboard, and internally we do the below calculation.
- We then assume half of the instance cost is for CPU and half for memory. So the effective rates used in the product are:
    - Price per core/hour = $0.029 ÷ 2 = $0.0145/hour
    - Price per GB/hour = $0.0145 ÷ 2 = $0.00725/hour

### Default prices

| Resource | Default used in calculations |
| --- | --- |
| CPU | **0.0145** $/core/hour |
| Memory | **0.00725** $/GB/hour |

### What prices are used for

All cost and savings figures (current cost, Current savings, possible savings etc.) use these hourly rates, converted to monthly amounts using 720 hours per month.

### Where you set it

- CPU and memory prices are configured in **Policies → Resource Pricing**.
- Values are stored in your browser and used only for cost calculations in the dashboard.
