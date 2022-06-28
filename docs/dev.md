# Provider Certification Tool
## Development Notes

This tool builds heavily on 
[Sonobuoy](https://sonobuoy.io) therefore at least
some high level knowledge of Sonobuoy is needed to really understand this tool. A 
good place to start with Sonobuoy is [its documentation](https://sonobuoy.io/docs).

The OpenShift provider certification tool extends Sonobuoy in two places:

- Command line interface (CLI)
- Plugins

### Command Line Interface

Sonobuoy provides its own CLI but it has a considerable number of flags and options
which can be overwhelming. This isn't an issue with Sonobuoy, it's just a symptom
of being a very flexible tool. However, for simplicity sake, the OpenShift
certification tool extends the Sonobuoy CLI with some strong opinions specific
to the realm certifying OpenShift on new infrastructure. 

#### Integration with Sonobuoy CLI
The OpenShift provider certification tool's CLI is written in Golang so that extending 
Sonobuoy could be done easily. Sonobuoy has two specific areas on which we build:

- Cobra commands (e.g. [sonobuoy run](https://github.com/vmware-tanzu/sonobuoy/blob/87e26ab7d2113bd32832a7bd70c2553ec31b2c2e/cmd/sonobuoy/app/run.go#L47-L62))
- Sonobuoy Client ([source code](https://github.com/vmware-tanzu/sonobuoy/blob/87e26ab7d2113bd32832a7bd70c2553ec31b2c2e/pkg/client/interfaces.go#L246-L250))

Some of this tool's commands will interact with the Sonobuoy Client directly
(this is ideal) but in some situations, like with this tools' `run` command, 
the similar Cobra command in Sonobuoy is used. This adds some odd interaction but
is necessarily as some complex code is present in Sonobuoy's Run command
implementation.

This direct usage of Sonobuoy's Cobra commands should be avoided since that
creates an odd development experience and the ability to cleanly set Sonobuoy's
flags is muddied by code like this:

```golang
// Not Great
runCmd.Flags().Set("dns-namespace", "openshift-dns")
runCmd.Flags().Set("kubeconfig", r.config.Kubeconfig)
```

Instead, use the Sonobuoy Client includes with the project like this:

```golang
// Great
reader, ec, err := config.SonobuoyClient.RetrieveResults(&client.RetrieveConfig{
    Namespace: "sonobuoy",
    Path:      config2.AggregatorResultsPath,
})
```

### Sonobuoy Plugins

*TODO* (Cert tool's plugin development is still in POC phase)

### Diagrams

*TODO* (These will be completed after code review and CLI structure is more finalized)

### Running Customized Certification Plugins

In some situations, you may need to modify the certification plugins that are run by the certification tool. 
Running the certification tool with customized plugin manifests cannot be used for final certification of an OpenShift cluster! 
If you find issues or changes that are needed for certification to complete, please open a GitHub issue or reach out to your Red Hat contact assisting with certification.  

1. Export default certification plugins to local filesystem:
```
openshift-provider-cert assets /tmp
INFO[2022-06-16T15:35:29-06:00] Asset openshift-conformance-validated.yaml saved to /tmp/openshift-conformance-validated.yaml 
INFO[2022-06-16T15:35:29-06:00] Asset openshift-kube-conformance.yaml saved to /tmp/openshift-kube-conformance.yaml 
```
2. Make your edits to the exported YAML assets:
```
vi /tmp/openshift-kube-conformance.yaml
```
3. Launch certification tool with customized plugin:
```
openshift-provider-cert run --plugin /tmp/openshift-kube-conformance.yaml --plugin /tmp/openshift-conformance-validated.yaml
```
