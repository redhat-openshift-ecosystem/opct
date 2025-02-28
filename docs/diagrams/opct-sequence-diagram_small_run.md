``` mermaid
%%{init: {"flowchart": {"useMaxWidth": false}}}%%

sequenceDiagram
  autonumber
  OPCT->>Server/Aggregator: opct run --watch [opts]
  loop Setup Workflow
    Server/Aggregator->>Server/Aggregator: Preflight Checks
    Server/Aggregator->>Server/Aggregator: Create Resources (RBAC, NS)
    Server/Aggregator->>Server/Aggregator: create server
  end
  Server/Aggregator->>Jobs: create/schedule jobs
  loop Init Job
    Jobs->>Jobs: Initialize job
    Jobs->>Jobs: Wait for Blocker job
    Jobs->>Server/Aggregator: report progress
    Jobs->>Jobs: Job/Plugin-N Unblocked
  end

  loop Start Job
    Jobs->>Jobs: openshift-tests run*
    Jobs->>Server/Aggregator: report progress
    Jobs->>Server/Aggregator: save results
  end

  loop End Workflow
    Jobs->>Jobs: collect artifacts
    Jobs->>Server/Aggregator: report progress
    Jobs->>Server/Aggregator: save results
    Server/Aggregator->>Server/Aggregator: Post Processor
    Server/Aggregator->>Server/Aggregator: Finished Artifacts
  end
```
