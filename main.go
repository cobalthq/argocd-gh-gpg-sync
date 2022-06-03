package main

import (
	"context"
	"encoding/json"
	"github.com/google/go-github/v45/github"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"log"
	"golang.org/x/oauth2"
	"os"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type signatureKey struct {
	KeyID string `json:"keyID"`
}

type signatureKeyPatch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value []signatureKey `json:"value"`
}

type configMapPatch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value map[string]string `json:"value"`
}

func patchKubernetes(ctx context.Context, gpgKeys map[string]string) {
	namespace := os.Getenv("ARGOCD_NAMESPACE")
	if namespace == "" {
		namespace = "argocd"
	}

	cmName := os.Getenv("ARGOCD_CONFIGMAP_NAME")
	if cmName == "" {
		cmName = "argocd-gpg-keys-cm"
	}

	projectName := os.Getenv("ARGOCD_PROJECT_NAME")
	if projectName == "" {
		projectName = "default"
	}

	config, err := rest.InClusterConfig()
	// create the clientset
	clientset, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("kubernetes.NewForConfig: %q", err.Error())
	}

	var signatureKeys []signatureKey
	for keyId, _ := range gpgKeys {
		signatureKeys = append(signatureKeys, signatureKey{KeyID: keyId})
	}

	skPayload := []signatureKeyPatch{{
		Op:    "replace",
		Path:  "/spec/signatureKeys",
		Value: signatureKeys,
	}}

	skPayloadBytes, _ := json.Marshal(skPayload)

	appGvr := schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "appprojects",
	}

	_, err = clientset.Resource(appGvr).Namespace(namespace).Patch(ctx, projectName, types.JSONPatchType, skPayloadBytes, v1.PatchOptions{})
	if err != nil {
		log.Fatalf("error patching project: %q\n", err)
	}

	cmPayload := []configMapPatch{{
		Op:    "replace",
		Path:  "/data",
		Value: gpgKeys,
	}}

	cmPayloadBytes, _ := json.Marshal(cmPayload)

	cmGvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	_, err = clientset.Resource(cmGvr).Namespace(namespace).Patch(ctx, cmName, types.JSONPatchType, cmPayloadBytes, v1.PatchOptions{})
	if err != nil {
		log.Fatalf("error patching configmap: %q\n", err)
	}

	log.Printf("Patched GPG key configmap and project with %d keys", len(gpgKeys))
}

func collectGpgKeys(ctx context.Context) map[string]string {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	gpgKeys := make(map[string]string)
	// Add GitHub's web-flow GPG key
	gpgKeys["4AEE18F83AFDEB23"] = `-----BEGIN PGP PUBLIC KEY BLOCK-----

xsBNBFmUaEEBCACzXTDt6ZnyaVtueZASBzgnAmK13q9Urgch+sKYeIhdymjuMQta
x15OklctmrZtqre5kwPUosG3/B2/ikuPYElcHgGPL4uL5Em6S5C/oozfkYzhwRrT
SQzvYjsE4I34To4UdE9KA97wrQjGoz2Bx72WDLyWwctD3DKQtYeHXswXXtXwKfjQ
7Fy4+Bf5IPh76dA8NJ6UtjjLIDlKqdxLW4atHe6xWFaJ+XdLUtsAroZcXBeWDCPa
buXCDscJcLJRKZVc62gOZXXtPfoHqvUPp3nuLA4YjH9bphbrMWMf810Wxz9JTd3v
yWgGqNY0zbBqeZoGv+TuExlRHT8ASGFS9SVDABEBAAHNNUdpdEh1YiAod2ViLWZs
b3cgY29tbWl0IHNpZ25pbmcpIDxub3JlcGx5QGdpdGh1Yi5jb20+wsBiBBMBCAAW
BQJZlGhBCRBK7hj4Ov3rIwIbAwIZAQAAmQEIACATWFmi2oxlBh3wAsySNCNV4IPf
DDMeh6j80WT7cgoX7V7xqJOxrfrqPEthQ3hgHIm7b5MPQlUr2q+UPL22t/I+ESF6
9b0QWLFSMJbMSk+BXkvSjH9q8jAO0986/pShPV5DU2sMxnx4LfLfHNhTzjXKokws
+8ptJ8uhMNIDXfXuzkZHIxoXk3rNcjDN5c5X+sK8UBRH092BIJWCOfaQt7v7wig5
4Ra28pM9GbHKXVNxmdLpCFyzvyMuCmINYYADsC848QQFFwnd4EQnupo6QvhEVx1O
j7wDwvuH5dCrLuLwtwXaQh0onG4583p0LGms2Mf5F+Ick6o/4peOlBoZz48=
=HXDP
-----END PGP PUBLIC KEY BLOCK-----`

	log.Printf("Collecting GPG keys from organization..")
	memberOpt := &github.ListMembersOptions{
		ListOptions: github.ListOptions{PerPage: 10},
	}
	for {
		users, resp, err := client.Organizations.ListMembers(ctx, os.Getenv("GITHUB_ORGANIZATION"), memberOpt)
		if err != nil {
			log.Fatalf("client.Organizations.ListMembers: %q", err)
		}

		for _, user := range users {
			gpgOpt := &github.ListOptions{
				PerPage: 10,
			}
			for {
				keys, gpgResp, err := client.Users.ListGPGKeys(ctx, *user.Login, gpgOpt)
				if err != nil {
					log.Fatalf("client.Users.ListGPGKeys: %q", err)
				}
				for _, key := range keys {
					if key.KeyID == nil || key.RawKey == nil {
						continue
					}
					gpgKeys[*key.KeyID] = *key.RawKey
				}
				if gpgResp.NextPage == 0 {
					break
				}
				gpgOpt.Page = gpgResp.NextPage
			}
		}

		if resp.NextPage == 0 {
			break
		}
		memberOpt.Page = resp.NextPage
		log.Printf("%d keys collected..", len(gpgKeys))
	}
	return gpgKeys
}

func main() {
	if os.Getenv("GITHUB_TOKEN") == "" {
		log.Fatalf("Missing required env var GITHUB_TOKEN")
	}

	if os.Getenv("GITHUB_ORGANIZATION") == "" {
		log.Fatalf("Missing required env var GITHUB_ORGANIZATION")
	}

	ctx := context.Background()
	gpgKeys := collectGpgKeys(ctx)
	patchKubernetes(ctx, gpgKeys)
}
