# argocd-gh-gpg-sync

This simple application runs as a CronJob inside your ArgoCD-enabled Kubernetes cluster
to synchronize all GPG keys from your GitHub organization into ArgoCD. It will add the
GPG keys to ArgoCD as well as set them as signature keys on a project.

With this you can enable GPG signature verification but not require your developers to
add their keys in two places.

## Usage

Deploy the `cobalthq/argocd-gh-gpg-sync` image as a CronJob anywhere in your cluster. You can use the following manifest
as an example. Note that it depends on a secret for the GitHub token.

```
apiVersion: v1
kind: ServiceAccount
metadata:
  name: argocd-gh-gpg-sync
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: argocd-gh-gpg-sync
subjects:
- kind: ServiceAccount
  name: argocd-gh-gpg-sync
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: argocd-gh-gpg-sync
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: argocd-gh-gpg-sync
rules:
  - apiGroups:
      - ""
    resources:
      - configmaps
    verbs:
      - patch
  - apiGroups:
      - argoproj.io
    resources:
      - appprojects
    verbs:
      - patch
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: argocd-gh-gpg-sync
spec:
  schedule: "*/15 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: argocd-gh-gpg-sync
          containers:
            - name: sync
              image: cobaltlabs/argocd-gh-gpg-sync:latest
              imagePullPolicy: Always
              env:
                - name: GITHUB_ORGANIZATION
                  value: cobalthq
                - name: GITHUB_TOKEN
                  valueFrom:
                    secretKeyRef:
                      name: argocd-github-creds # You will need to provide this secret!
                      key: GITHUB_TOKEN
          restartPolicy: OnFailure
```

### Environment Variables
| Variable              | Purpose                                                                      |
|-----------------------|------------------------------------------------------------------------------|
| GITHUB_TOKEN          | A GitHub personal access token used to read the members of your organization |
| GITHUB_ORGANIZATION   | Which organization should be imported                                        |
| ARGOCD_NAMESPACE      | Namespace that ArgoCD resides in (default 'argocd')                          |
| ARGOCD_CONFIGMAP_NAME | Name of ArgoCD's GPG key configmap (default 'argocd-gpg-keys-cm')            |
| ARGOCD_PROJECT_NAME   | Name of project to import keys to (default 'default')                        |

## Limitations
* Keys can only be imported to one project.
* All GitHub organization members, regardless of team, will be imported.