## Structure

The `config` folder contains the Kustomize resources that are used to assemble the operator's deployment units

```
.
├── certmanager   # Kustomize options dealing with cert-manager
├── crd           # Kustomize options for our CRDs
│   ├── bases     # auto generated based on the code annotations (`make manifests`)
│   └── patches   # patches to apply to the generated CRD
├── default       # Kustomize's "entry point", generating the distribution YAML file
├── manager       # the operator's Deployment
├── rbac          # RBAC rules
└── webhook       # Webhook configuration and service
```
