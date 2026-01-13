# Dynamic Kustomize Build Path

`gitops-kustomzchk` works by requiring a Kustomize Build as step 1, it produces full k8s manifests of before state and after state, then carry out some difference comparision. As of now (v0.4), the input of `gitops-kustomzchk` regarding of Kustomize Build command only has flag `--manifests-path` and `--environments` and `--service` to control the location of the build (aka. the location of the `kustomization.yaml` file)

## Current Implementation (v0.4 and prior)

For example, if normally, we would run the following command to build full manifest:

```
kustomize build /my-k8s-manifests/services/my-app/environment/stg
```

This implies that there exists `/my-k8s-manifests/services/my-app/environment/stg/kustomization.yaml`

We can break the path down into:

* From: `kustomize build /my-k8s-manifests/services/my-app/environment/stg`
* To: `kustomize build <MANIFESTS_PATH>/<SERVICE>/<KUSTOMIZE_OVERLAY_DIR_NAME>/<ENVIRONMENT_NAME>
  Such that:
  * `<MANIFESTS_PATH>` is the input flag `--manifests-path` (and in this example: `/my-k8s-manifests/services`)
  * `<SERVICE>` is the input flag `--service` (and in this example: `my-app`)
  * `<KUSTOMIZE_OVERLAY_DIR_NAME>` is hardcoded in the builder.go logic and currently is `"environments"`
  * `<ENVIRONMENT_NAME>` is "dirname" of the overlay, in our case, we use `stg` and `prod`, but in reality, it could be anything. This value is the input flag `--environments` and it accept a list of comma-separated strings. (and in this example: `stg`, or even `stg,prod` if prod overlays exists for that service)

The above is true for the prior and current v0.4 implementation of `src/pkg/kustomize/builder.go`

## Proposed Implementation

### Arosen Situation

In the above, we deliberately **name** parts of the kustomize build path, but it is case-by-case for everyone. We must generalize it so it can be applicable for a wider audience.

Actually, in our live environment, we have evolved from

```
/my-k8s-manifests/services/my-app/environment/stg/
```

To

```
/my-k8s-manifests/services/my-app/clusters/alpha/stg/
```

Meaning, `"alpha"` and `"stg"` are both overlays!

### Adapted Proposal

Kustomize doesn't enforce what name of overlay you must use, so I think we should keep this spirit too. I'm thinking of doing the following:

```
--kustomize-build-path "/home/me/my-k8s-manifests/services/$SERVICE/overlays_dir1/$OVERLAY1/overlays_dir2/$OVERLAY2"
--kustomize-build-values "SERVICE=my-app1;OVERLAY1=alpha,beta;OVERLAY2=stg,prod"
```

The program then parse the build path for variables with dollar prefix `$SERVICE`, then look for value that variable will hold in the `--kustomize-build-values` string.

Some contraints:
- `--kustomize-build-path` must be valid path if we replace `$VARIABLE` with the actual value (meaning no more $ sign)
- `--kustomize-build-values` is a string of many `KEY=value1,value` token separated by `;`. Each token follows `KEY=value1,value2` key-value format, if only 1 value, write only `value1`, if multiple value, use comma-separate string

This proposal would change many logic in `src/pkg/models` as well for example, in the generated Diff report or Policy report. For each report, we would leave out all hardcoded info, and just use the full interpolated kustomize build path. So, instead of

```
Diff Report of Environment `stg`
```

It would be

```
Diff Report of `my-app/overlay_dir1/alpha/overlay_dir2/stg`
```

This seems less of an headache.

<TODO: Deeper investigation of this design would cause any incompatibility or change massively the report render and policy evaluation>