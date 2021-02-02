package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mapset "github.com/deckarep/golang-set"
	"github.com/sirupsen/logrus"
	"k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestGetTaintsToAdd(t *testing.T) {

	containerRequestingCPU := core.Container{
		Resources: core.ResourceRequirements{
			Requests: core.ResourceList{
				core.ResourceCPU: *resource.NewQuantity(2, resource.DecimalSI),
			},
		},
	}

	containerRequestingMemory := core.Container{
		Resources: core.ResourceRequirements{
			Requests: core.ResourceList{
				core.ResourceMemory: *resource.NewQuantity(2048, resource.DecimalSI),
			},
		},
	}

	targetExtendedResource1 := "sunny.com/device-sunny"
	targetExtendedResource2 := "sunny.com/device-cloud"

	containerRequestingExtendedResource1 := core.Container{
		Resources: core.ResourceRequirements{
			Requests: core.ResourceList{
				core.ResourceName(targetExtendedResource1): *resource.NewQuantity(1, resource.DecimalSI),
			},
		},
	}
	containerRequestingExtendedResource2 := core.Container{
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

	targetExtendedResource1 := "sunny.com/device-sunny"
	targetExtendedResource2 := "sunny.com/device-cloud"

	var targetResources ArrayFlags
	targetResources.Set(targetExtendedResource1)
	targetResources.Set(targetExtendedResource2)

	SetTargetResourcesSet(targetResources)

	testServer := httptest.NewServer(GetAdmissionWebhookServerNoTls(8080).Handler)

	test := v1beta1.AdmissionReview{
		TypeMeta: v1.TypeMeta{
			Kind: "AdmissionReview",
		},
		Request: &v1beta1.AdmissionRequest{
			UID: "31390b02-650e-11eb-ae93-0242ac130002",
			Kind: v1.GroupVersionKind{
				Kind: "Pod",
			},
			Operation: "CREATE",
			Object: runtime.RawExtension{
				Raw: []byte(`{"metadata": {
								"name": "test",
								"uid": "31390b02-650e-11eb-ae93-0242ac130002",
								"creationTimestamp": "2018-09-28T12:20:39Z"
							  }}`),
			},
		},
	}

	requestString := string(encodeRequest(&test))
	requestData := strings.NewReader(requestString)

	r, _ := http.Post(testServer.URL, "application/json", requestData)
	response := decodeResponse(r.Body)

	println("HTTP Request response : ", response)

	// if response.Request.UID != AdmissionRequestNS.Request.UID {
	// 	t.Error("Request and response UID don't match")
	// }
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

func encodeRequest(review *v1beta1.AdmissionReview) []byte {
	data, err := json.Marshal(review)

	if err != nil {
		logrus.Errorln(err)
	}

	return data
}

func decodeResponse(body io.ReadCloser) *v1beta1.AdmissionReview {
	response, _ := ioutil.ReadAll(body)

	review := &v1beta1.AdmissionReview{}
	Codecs.UniversalDeserializer().Decode(response, nil, review)

	return review
}

// func generateKey() {
//     privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
//     if err != nil {
//         fmt.Printf("Cannot generate RSA key\n")
//         os.Exit(1)
// 	}

//     publicKey := &privateKey.PublicKey
// }
