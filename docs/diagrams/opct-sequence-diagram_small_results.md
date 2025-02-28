``` mermaid
%%{init: {"flowchart": {"useMaxWidth": false}}}%%

sequenceDiagram
  autonumber
  OPCT->>Server/Aggregator: opct retrieve
  OPCT->>Server/Aggregator: opct destroy
  Server/Aggregator->>OPCT: Finished
  OPCT->>OPCT: opct report -s ./report <result>.tar.gz
  OPCT->>OPCT: open http://127.0.0.1:9090
```
