# Review Results with AI Assistant

The OPCT review flow creates a consolidated report of the conformance workflow. There are different ways you can interact with the OPCT review flow:

- CLI report
- Web UI report
- Assisted report

This guide shows you how to leverage an LLM to explore the workflow results processed by OPCT in a conversational interface using Claude. This feature is called the Assisted report.

To begin exploring the CLI and Web UI report command, take a look at the [`opct report` command reference](https://redhat-openshift-ecosystem.github.io/opct/opct/report/).


## Assisted report

The Assisted report is an additional helper to explore the results, troubleshoot existing issues, and guide you through action items to fix any issues reported in the results.

The Assisted report uses Claude AI to provide an enhanced LLM-powered conversational interface for the analysis process.

Please make sure you have Claude configured in your environment and the `claude` CLI is available in your PATH.

### Prerequisites

- Claude CLI installed
- Claude environment configured
!!! tip "Red Hat Employee only"
    Red Hatters can use the [quick step guide](https://docs.google.com/document/d/1eNARy9CI28o09E7Foq01e5WD5MvEj3LSBnXqFcprxjo/edit?tab=t.0#heading=h.8ldy5by9bpo8) to setup Claude environment.
- Model used: Claude 4.5 Sonnet or Claude 4 Sonnet (used in this tutorial)
- OPCT installed
- OPCT results collected and available in your current directory

### Generate the OPCT report

Extract the results archive and generate the report:

```sh
RESULT_FILE=opct_202510091157_67175c7a-0cc0-42c2-877f-d2652daa75fc.tar.gz
RESULT_DIR=$(basename -s .tar.gz $RESULT_FILE)
mkdir ${RESULT_DIR}
opct report -s ./${RESULT_DIR} ${RESULT_FILE}
```

- Check the output and verify that the Web UI is available at http://localhost:9090

- Keep the Web UI / opct report command running

### Explore the results with Claude

The following steps will set up a dedicated working directory for Claude with the necessary configuration files.

- Create a working directory and download the Claude configuration:

```sh
# Create dedicated directory for Claude workspace (optional)
CLAUDE_WORKDIR=${RESULT_DIR}-assisted
mkdir ${CLAUDE_WORKDIR}
cd ${CLAUDE_WORKDIR}

# Download Claude configuration for OPCT
wget https://raw.githubusercontent.com/redhat-openshift-ecosystem/opct/refs/heads/main/docs/devel/review/assistant-assets/settings.local.json
wget https://raw.githubusercontent.com/redhat-openshift-ecosystem/opct/refs/heads/main/docs/devel/review/assistant-assets/system.prompt.txt
```

> **Note:** You can also manually copy the [`system.prompt.txt`](./assistant-assets/system.prompt.txt) and [`settings.local.json`](./assistant-assets/settings.local.json) files instead of downloading them.

- Run the Claude CLI in interactive mode:

```sh
claude \
--append-system-prompt "${PWD}/system.prompt.txt" \
--settings "${PWD}/settings.local.json" \
--strict-mcp-config \
--mcp-debug
```

!!! warning "Important"
    The `--add-dir` flag grants Claude access to the results directory. It was intentionally not used in favor of fetching structured data through OPCT Web UI, instead of file discovery/filter method from large files.

### Review Prompts

Now that the Claude environment is configured with instructions to explore OPCT results through the system prompt, you can explore the test results in interactive mode. The following prompts provide examples to help you troubleshoot issues collected by OPCT results from a conformance execution:

#### Get execution summary

Check the overall summary of the execution:

```
Show the summary of the conformance workflow with immediate action items
```

#### Analyze test failures by component

Explore test failures from results, grouping by test group and troubleshooting the test failure output:

!!! warning "High Tokens Utilization"
    This prompt will consume many tokens as it walks through large log files of test failures.

```
Provide a summary of e2e failures by priority and component/group, checking each log to find the issue. The OPCT results provide the failed tests as well as the log failures. Provide the exact test name impacted in each group.
```

#### Investigate specific check failures

Explore a specific check failure:

```
how to resolve the OPCT-010A and tell me why this failed in my environment?
```

Explore a specific test failure:

```
explain why the following test failed in the openshift conformance job and investigate how to fix: [sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster shouldn't report any alerts in firing state apart from Watchdog and AlertmanagerReceiversNotConfigured [Early][apigroup:config.openshift.io] [Skipped:Disconnected] [Suite:openshift/conformance/parallel]
```
