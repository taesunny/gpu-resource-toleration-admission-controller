package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mapset "github.com/deckarep/golang-set"
	admissionv1 "k8s.io/api/admission/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// test example cmd : go test -v .\webhook\
func TestGetTaintsToAdd(t *testing.T) {

	containerRequestingCPU := core.Container{
		Name: "test-cpu-only-container",
		Resources: core.ResourceRequirements{
			Requests: core.ResourceList{
				core.ResourceCPU: *resource.NewQuantity(2, resource.DecimalSI),
			},
		},
	}

	containerRequestingMemory := core.Container{
		Name: "test-memory-only-container",
		Resources: core.ResourceRequirements{
			Requests: core.ResourceList{
				core.ResourceMemory: *resource.NewQuantity(2048, resource.DecimalSI),
			},
		},
	}

	targetExtendedResource1 := "sunny.com/device-sunny"
	targetExtendedResource2 := "sunny.com/device-cloud"

	containerRequestingExtendedResource1 := core.Container{
		Name: "test-extended-resource-type1-container",
		Resources: core.ResourceRequirements{
			Requests: core.ResourceList{
				core.ResourceName(targetExtendedResource1): *resource.NewQuantity(1, resource.DecimalSI),
			},
		},
	}
	containerRequestingExtendedResource2 := core.Container{
		Name: "test-extended-resource-type1-container",
		Resources: core.ResourceRequirements{
			Requests: core.ResourceList{
				core.ResourceName(targetExtendedResource2): *resource.NewQuantity(2, resource.DecimalSI),
			},
		},
	}

	var targetResources ArrayFlags
	targetResources.Set(targetExtendedResource1)
	targetResources.Set(targetExtendedResource2)

	SetTargetResourcesSet(targetResources)

	tests := []struct {
		description              string
		requestedPod             core.Pod
		expectedTolerationsToAdd mapset.Set
	}{
		{
			description: "empty pod without any extended resources, expect no change in tolerations",
			requestedPod: core.Pod{
				Spec: core.PodSpec{},
			},
			expectedTolerationsToAdd: mapset.NewSet(),
		},
		{
			description: "pod with container without any extended resources, expect no change in tolerations",
			requestedPod: core.Pod{
				Spec: core.PodSpec{
					Containers: []core.Container{
						containerRequestingCPU,
					},
				},
			},
			expectedTolerationsToAdd: mapset.NewSet(),
		},
		{
			description: "pod with init container without any extended resources, expect no change in tolerations",
			requestedPod: core.Pod{
				Spec: core.PodSpec{
					InitContainers: []core.Container{
						containerRequestingMemory,
					},
				},
			},
			expectedTolerationsToAdd: mapset.NewSet(),
		},
		{
			description: "pod with container with extended resource, expect toleration to be added",
			requestedPod: core.Pod{
				Spec: core.PodSpec{
					Containers: []core.Container{
						containerRequestingExtendedResource1,
					},
				},
			},
			expectedTolerationsToAdd: mapset.NewSet(targetExtendedResource1),
		},
		{
			description: "pod with init container with extended resource, expect toleration to be added",
			requestedPod: core.Pod{
				Spec: core.PodSpec{
					InitContainers: []core.Container{
						containerRequestingExtendedResource2,
					},
				},
			},
			expectedTolerationsToAdd: mapset.NewSet(targetExtendedResource2),
		},
		{
			description: "pod with existing tolerations and container with extended resource, expect existing tolerations to be preserved and new toleration to be added",
			requestedPod: core.Pod{
				Spec: core.PodSpec{
					Containers: []core.Container{
						containerRequestingCPU,
						containerRequestingExtendedResource1,
					},
					Tolerations: []core.Toleration{
						{
							Key:      "foo",
							Operator: core.TolerationOpEqual,
							Value:    "bar",
							Effect:   core.TaintEffectNoSchedule,
						},
					},
				},
			},
			expectedTolerationsToAdd: mapset.NewSet(targetExtendedResource1),
		},
		{
			description: "pod with multiple extended resources, expect multiple tolerations to be added",
			requestedPod: core.Pod{
				Spec: core.PodSpec{
					Containers: []core.Container{
						containerRequestingMemory,
						containerRequestingExtendedResource1,
					},
					InitContainers: []core.Container{
						containerRequestingCPU,
						containerRequestingExtendedResource2,
					},
				},
			},
			expectedTolerationsToAdd: mapset.NewSet(targetExtendedResource1, targetExtendedResource2),
		},
		{
			description: "pod with container requesting extended resource and existing correct toleration, expect no change in tolerations",
			requestedPod: core.Pod{
				Spec: core.PodSpec{
					Containers: []core.Container{
						containerRequestingCPU,
						containerRequestingMemory,
						containerRequestingExtendedResource1,
					},
					Tolerations: []core.Toleration{
						{
							Key:      targetExtendedResource1,
							Operator: core.TolerationOpExists,
							Effect:   core.TaintEffectNoSchedule,
						},
					},
				},
			},
			expectedTolerationsToAdd: mapset.NewSet(),
		},
		{
			description: "pod with container requesting extended resource and existing toleration with the same key but different effect and value, expect existing tolerations to be preserved and do nothing yet....",
			requestedPod: core.Pod{
				Spec: core.PodSpec{
					Containers: []core.Container{
						containerRequestingCPU,
						containerRequestingMemory,
						containerRequestingExtendedResource1,
					},
					Tolerations: []core.Toleration{
						{
							Key:      targetExtendedResource1,
							Operator: core.TolerationOpEqual,
							Value:    "foo",
							Effect:   core.TaintEffectNoExecute,
						},
					},
				},
			},
			expectedTolerationsToAdd: mapset.NewSet(),
		},
		{
			description: "pod with wildcard toleration and container requesting extended resource, expect existing tolerations to be preserved and new toleration to be added",
			requestedPod: core.Pod{
				Spec: core.PodSpec{
					Containers: []core.Container{
						containerRequestingCPU,
						containerRequestingMemory,
						containerRequestingExtendedResource1,
					},
					Tolerations: []core.Toleration{
						{
							Operator: core.TolerationOpExists,
						},
					},
				},
			},
			expectedTolerationsToAdd: mapset.NewSet(targetExtendedResource1),
		},
	}

	for _, test := range tests {
		tolerationsToAdd := GetExtendResourceTolerationsToAdd(&test.requestedPod)

		if test.expectedTolerationsToAdd.Equal(*tolerationsToAdd) {
			// t.Logf("Test (%s) Succeed", test.description)
		} else {
			println("original pod toleration list: ", test.requestedPod.Spec.Tolerations)
			println("expected: ", test.expectedTolerationsToAdd.String())
			println("return of function: ", (*tolerationsToAdd).String())
			println()
			t.Errorf("Test (%s) Failed", test.description)
		}
	}
}

func TestHttpRequest(t *testing.T) {
	// keyPair, err := tls.LoadX509KeyPair("./manifests/sunny.crt", "./manifests/sunny.key")
	// if err != nil {
	// 	klog.Errorf("Failed to load key pair: %s", err)
	// }

	containerRequestingCPU := core.Container{
		Name: "test-only-cpu-container",
		Resources: core.ResourceRequirements{
			Requests: core.ResourceList{
				core.ResourceCPU: *resource.NewQuantity(2, resource.DecimalSI),
			},
		},
	}

	containerRequestingMemory := core.Container{
		Name: "test-only-memory-container",
		Resources: core.ResourceRequirements{
			Requests: core.ResourceList{
				core.ResourceMemory: *resource.NewQuantity(2048, resource.DecimalSI),
			},
		},
	}

	targetExtendedResource1 := "sunny.com/device-sunny"
	targetExtendedResource2 := "sunny.com/device-cloud"

	containerRequestingExtendedResource1 := core.Container{
		Name: "test-extended-resource-type1-container",
		Resources: core.ResourceRequirements{
			Requests: core.ResourceList{
				core.ResourceName(targetExtendedResource1): *resource.NewQuantity(1, resource.DecimalSI),
			},
		},
	}
	containerRequestingExtendedResource2 := core.Container{
		Name: "test-extended-resource-type2-container",
		Resources: core.ResourceRequirements{
			Requests: core.ResourceList{
				core.ResourceName(targetExtendedResource2): *resource.NewQuantity(2, resource.DecimalSI),
			},
		},
	}

	var targetResources ArrayFlags
	targetResources.Set(targetExtendedResource1)
	targetResources.Set(targetExtendedResource2)

	SetTargetResourcesSet(targetResources)

	tests := []struct {
		description              string
		requestedPod             core.Pod
		expectedTolerationsToAdd mapset.Set
	}{
		{
			description: "empty pod without any extended resources, expect no change in tolerations",
			requestedPod: core.Pod{
				Spec: core.PodSpec{},
			},
			expectedTolerationsToAdd: mapset.NewSet(),
		},
		{
			description: "pod with container without any extended resources, expect no change in tolerations",
			requestedPod: core.Pod{
				Spec: core.PodSpec{
					Containers: []core.Container{
						containerRequestingCPU,
					},
				},
			},
			expectedTolerationsToAdd: mapset.NewSet(),
		},
		{
			description: "pod with init container without any extended resources, expect no change in tolerations",
			requestedPod: core.Pod{
				Spec: core.PodSpec{
					InitContainers: []core.Container{
						containerRequestingMemory,
					},
				},
			},
			expectedTolerationsToAdd: mapset.NewSet(),
		},
		{
			description: "pod with container with extended resource, expect toleration to be added",
			requestedPod: core.Pod{
				Spec: core.PodSpec{
					Containers: []core.Container{
						containerRequestingExtendedResource1,
					},
				},
			},
			expectedTolerationsToAdd: mapset.NewSet(targetExtendedResource1),
		},
		{
			description: "pod with init container with extended resource, expect toleration to be added",
			requestedPod: core.Pod{
				Spec: core.PodSpec{
					InitContainers: []core.Container{
						containerRequestingExtendedResource2,
					},
				},
			},
			expectedTolerationsToAdd: mapset.NewSet(targetExtendedResource2),
		},
	}

	testServer := httptest.NewServer(GetAdmissionWebhookServerNoTls(8080).Handler)

	for _, test := range tests {
		log.Printf("Test: %s\n", test.description)

		podData, err := json.Marshal(test.requestedPod)

		if err != nil {
			log.Println(err)
		}

		ar := admissionv1.AdmissionReview{
			TypeMeta: v1.TypeMeta{
				Kind: "AdmissionReview",
			},
			Request: &admissionv1.AdmissionRequest{
				UID: "31390b02-650e-11eb-ae93-0242ac130002",
				Kind: v1.GroupVersionKind{
					Kind: "Pod",
				},
				Operation: "CREATE",
				Object: runtime.RawExtension{
					Raw: []byte(podData),
				},
			},
		}

		requestString := string(encodeRequest(&ar))
		requestData := strings.NewReader(requestString)

		r, _ := http.Post(testServer.URL+"/mutate", "application/json", requestData)
		review, response := decodeResponse(r.Body)

		if review.Response.Patch == nil {
			log.Println("review.Response.Patch is nil")
			continue
		}

		var patch []PatchOps

		if err := json.Unmarshal(review.Response.Patch, &patch); err != nil {
			log.Fatalf("Unmarshal failed!!! HTTP Request r.Status : %s\nerr:%s", r.Status, err)
		}

		if patch == nil {
			log.Println("unmarshaled patch is nil")
			continue
		}

		if !review.Response.Allowed || !isValidPatchData(&patch, &test.expectedTolerationsToAdd) {
			log.Printf("Test Failed : %s\n", test.description)

			log.Printf("Expected Tolerations : %s\n", test.expectedTolerationsToAdd)

			log.Println("HTTP Request r.Status : ", r.Status)
			log.Println("HTTP Request r.StatusCode : ", r.StatusCode)
			log.Println("HTTP Request r.Body : ", response)
			log.Println("HTTP Request review.Response.UID : ", review.Response.UID)
			log.Println("HTTP Request review.Response.Allowed : ", review.Response.Allowed)

			if review.Response.Patch != nil {
				log.Println("HTTP Request review.Response.Patch : ", string(review.Response.Patch))
			} else {
				log.Println("HTTP Request review.Response.Patch is nil")
			}

			if review.Response.Result != nil {
				log.Println("HTTP Request review.Response.Result : ", review.Response.Result.Message)
			} else {
				log.Printf("HTTP Request review.Response.Result is nil")
			}
		}
	}
}

func GetAdmissionWebhookServerNoTls(port int) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", HandleMutate)
	mux.HandleFunc("/validate", HandleValidate)

	webhookServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return webhookServer
}

func encodeRequest(review *admissionv1.AdmissionReview) []byte {
	data, err := json.Marshal(review)

	if err != nil {
		log.Println(err)
	}

	return data
}

func decodeResponse(body io.ReadCloser) (*admissionv1.AdmissionReview, []byte) {
	response, _ := ioutil.ReadAll(body)

	review := &admissionv1.AdmissionReview{}
	Codecs.UniversalDeserializer().Decode(response, nil, review)

	return review, response
}

func isValidPatchData(patchData *[]PatchOps, expectedPatches *mapset.Set) bool {
	var patchOpsMap mapset.Set = mapset.NewSet()

	for _, patch := range *patchData {
		if patch.Value == nil {
			continue
		}

		var tolerations []interface{} = (patch.Value).([]interface{})

		for _, toleration := range tolerations {
			tolerationMap := toleration.(map[string]interface{})
			key := tolerationMap["key"].(string)

			if key == "" {
				log.Fatalf("Getting tolerationMap[key] failed\n")
			}

			patchOpsMap.Add(key)
		}
	}

	return patchOpsMap.Equal(*expectedPatches)
}

// func generateKey() {
//     privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
//     if err != nil {
//         fmt.Printf("Cannot generate RSA key\n")
//         os.Exit(1)
// 	}

//     publicKey := &privateKey.PublicKey
// }
