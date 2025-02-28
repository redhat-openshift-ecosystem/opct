---
title: Website architecture
---

## Technology

OPCT documentation uses [mkdocs](https://www.mkdocs.org/) for static site generation.

## Source Code

The website is located in the `docs/` directory of the [opct repository](https://github.com/redhat-openshift-ecosystem/provider-certification-tool).

## Theme

This site is based on [@material](https://squidfunk.github.io/mkdocs-material/) theme, with
customizations defined under the `theme` section in the `mkdocs.yaml` file under the root
directory of the repository.

## Navigation

The left menu is configured in the `nav` section of `mkdocs.yaml`.

## Diagrams as code

`mkdocs` plugins are defined under the `plugins` section in `mkdocs.yaml`.

### Using `diagrams`

You can write diagrams as code using the Python language with the library
[`diagrams`](https://diagrams.mingrammer.com/), the Python file must be located in the
`docs/` directory and have the suffix `.diagram.py`.

The image defined in `filename` can be used directly in your markdown file; it is
rendered when the site is built or served locally.

For example, define the image name in the `filename` attribute of your `Diagram` in a
file `docs/diagrams/my-diagram.diagram.py`:

```py
with Diagram("OCP/OKD Cluster", show=False, filename="./cluster-example"):
```

The image `docs/diagrams/cluster-example.png` when you run `mkdocs build` or `mkdocs serve`.

You can reference the image in your markdown file `docs/diagrams/my-doc.md`, such as:

```md
![OCP Cluster Reference](./cluster-example.png)
```

### Mermaid.js

You also can draw diagram as a code with [`Mermaid.js`](https://mermaid.js.org/)
directly in markdown files.

The mkdocs plugins [`diagrams`](https://squidfunk.github.io/mkdocs-material/reference/diagrams/)
enables native support for Mermaid.js diagrams.

Material for MkDocs will automatically initialize the JavaScript
runtime when a page includes a mermaid code block


## Articles

Articles/Guides are located in `docs/guides` in `*.md` files.

## Hosting

We use GitHub Pages as static website hosting and CD.

GitHub deploys the website to production after merging anything to a `main` branch.

## Local Testing

Install `mkdocs` and dependencies.

Run:

```sh
pip install -r docs/requirements.txt
mkdocs serve
```

And navigate to `http://localhost:8000` after successful build.
There is no need to restart mkdocs server almost for all changes as it supports hot reload.
Also, there is no need to refresh a webpage as hot reload updates changed content on the open page.

## Website Build

To do it run:

```sh
make build-docs
```
