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
