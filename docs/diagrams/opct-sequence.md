---
hide:
  - navigation
  - toc
---

# OPCT Execution Flow

Diagram describing the default OPCT execution flow (sequence).

``` mermaid
%%{init: {"flowchart": {"useMaxWidth": false}}}%%

sequenceDiagram
  autonumber
  OPCT->>OCP/KAS: ./opct run -w [opts]
  loop Setup
      OCP/KAS->>OCP/KAS: Preflight Checks
      OCP/KAS->>OCP/KAS: Create Resources (RBAC, NS)
  end
  OCP/KAS->>Sonobuoy: create server
  Sonobuoy->>Plugins: create plugins
  loop Init
      Plugins->>Plugins: Extract utilities
      Plugins->>Plugins: Wait for Blocker plugin
      Plugins->>Sonobuoy: report progress
  end
  Plugins->>Plugins: Plugin-N Unblocked
  Plugins->>P_Upgrade: run cluster upgrade*
  Note right of Plugins: *--mode=upgrade
  P_Upgrade->>Sonobuoy: report progress
  P_Upgrade->>Sonobuoy: save results
  Plugins->>P_Conformance: run conformance plugins: kubernetes, openshift
  P_Conformance->>P_Conformance: kubernetes e2e tests
  P_Conformance->>Sonobuoy: report progress
  P_Conformance->>Sonobuoy: save results

  P_Conformance->>P_Conformance: openshift e2e tests
  P_Conformance->>Sonobuoy: report progress
  P_Conformance->>Sonobuoy: save results

  Plugins->>P_Artifacts: run plugin: collect artifacts
  P_Artifacts->>Sonobuoy: report progress
  P_Artifacts->>Sonobuoy: save results
  Sonobuoy->>OCP/KAS: collect cluster objects
  Sonobuoy->>Sonobuoy: Post Processor
  Sonobuoy->>Sonobuoy: Finished Artifacts
  Sonobuoy->>OPCT: Show Summary
  OPCT->>Sonobuoy: ./opct retrieve
  OPCT->>OPCT: ./opct results <result>.tar.gz
  OPCT->>OCP/KAS: ./opct destroy
  OCP/KAS->>OPCT: Finished
```
