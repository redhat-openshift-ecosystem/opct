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
  Sonobuoy->>Jobs/Plugins: create/schedule jobs
  loop Init
      Jobs/Plugins->>Jobs/Plugins: Extract utilities
      Jobs/Plugins->>Jobs/Plugins: Wait for Blocker job
      Jobs/Plugins->>Sonobuoy: report progress
  end
  Jobs/Plugins->>Jobs/Plugins: Job/Plugin-N Unblocked
  Jobs/Plugins->>Job/P_Upgrade: run cluster upgrade*
  Note right of Jobs/Plugins: *--mode=upgrade
  Job/P_Upgrade->>Sonobuoy: report progress
  Job/P_Upgrade->>Sonobuoy: save results
  Jobs/Plugins->>Job/P_Conformance: run conformance jobs: kubernetes, openshift
  Job/P_Conformance->>Job/P_Conformance: kubernetes e2e tests
  Job/P_Conformance->>Sonobuoy: report progress
  Job/P_Conformance->>Sonobuoy: save results

  Job/P_Conformance->>Job/P_Conformance: openshift e2e tests
  Job/P_Conformance->>Sonobuoy: report progress
  Job/P_Conformance->>Sonobuoy: save results

  Jobs/Plugins->>Job/P_Artifacts: run plugin: collect artifacts
  Job/P_Artifacts->>Sonobuoy: report progress
  Job/P_Artifacts->>Sonobuoy: save results
  Sonobuoy->>OCP/KAS: collect cluster objects
  Sonobuoy->>Sonobuoy: Post Processor
  Sonobuoy->>Sonobuoy: Finished Artifacts
  Sonobuoy->>OPCT: Show Summary
  OPCT->>Sonobuoy: ./opct retrieve
  OPCT->>OPCT: ./opct results <result>.tar.gz
  OPCT->>OCP/KAS: ./opct destroy
  OCP/KAS->>OPCT: Finished
```
