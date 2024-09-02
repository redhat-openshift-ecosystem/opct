# OPCT quick start

To quick start with the OPCT CLI, follow these steps:

1. Install the OPCT CLI by running the following command:
    ```
    wget -O ~/bin/opct https://github.com/redhat-openshift-ecosystem/provider-certification-tool/releases/latest/download/opct-linux-amd64
    chmod u+x ~/bin/opct
    ```

2. Select a node to be dedicated to host the workflow without disruptions:
    ```
    opct adm setup-node
    ```

3. Run the default workflow monitoring the execution:
    ```
    opct run --watch
    ```

4. Retrieve the results:
    ```
    opct retrieve
    ```

5. Explore the results:
    ```
    opct report opct*.tar.gz
    ```

6. Explore the report UI, read the Checks, recommendations, and logs.


That's it! You have successfully set up and run the OPCT CLI to perform conformance test suites,
and collected performance data from your environment.
