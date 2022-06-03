# argocd-gh-gpg-sync

This simple application runs as a CronJob inside your ArgoCD-enabled Kubernetes cluster
to synchronize all GPG keys from your GitHub organization into ArgoCD. It will add the
GPG keys to ArgoCD as well as set them as signature keys on a project.

With this you can enable GPG signature verification but not require your developers to
add their keys in two places.

## Usage

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