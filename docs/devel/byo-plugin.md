# BYO Plugin

!!! danger "Experimental"
    BYO Plugin on OPCT is experimental and non-supported.
    If you want to explore, take a look at the
    Sonobuoy Plugins documentation, and the command wrapper
    `opct sonobuoy`.

## Customize Plugins <a name="byo-plugin-customize"></a>

In some situations, you may need to modify the plugins that are run by the OPCT.

< Running the OPCT with customized plugin manifests cannot be used for final validation of an OpenShift cluster!

If you find issues or changes that are needed to complete, please open a 
GitHub issue or reach out to your Red Hat contact assisting with validation process.

Steps:

1. Export default plugins to local filesystem:
```
$ ./opct assets /tmp
INFO[2022-06-16T15:35:29-06:00] Asset openshift-conformance-validated.yaml saved to /tmp/openshift-conformance-validated.yaml 
INFO[2022-06-16T15:35:29-06:00] Asset openshift-kube-conformance.yaml saved to /tmp/openshift-kube-conformance.yaml 
```
2. Make your edits to the exported YAML assets:
```
vi /tmp/openshift-kube-conformance.yaml
```
3. Launch the tool with customized plugin:
```
./opct run --plugin /tmp/openshift-kube-conformance.yaml --plugin /tmp/openshift-conformance-validated.yaml
```

<!-- ## BYO Plugin from scratch

> TBD -->
