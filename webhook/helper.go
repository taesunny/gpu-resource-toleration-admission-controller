package webhook

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	mapset "github.com/deckarep/golang-set"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	RuntimeScheme = runtime.NewScheme()
	Codecs        = serializer.NewCodecFactory(RuntimeScheme)
	Deserializer  = Codecs.UniversalDeserializer()
)

type ArrayFlags []string

func (i *ArrayFlags) String() string {
	// return strings.Trim(fmt.Sprint(i), "[]")
	return ""
}

func (i *ArrayFlags) Set(value string) error {
	*i = append(*i, strings.TrimSpace(value))
	return nil
}

var targetResourcesSet mapset.Set

func SetTargetResourcesSet(targetResources ArrayFlags) {
	targetResourcesSet = mapset.NewSet()

	for _, resource := range targetResources {
		targetResourcesSet.Add(resource)
	}
}

func GetTargetResourcesSet() *mapset.Set {
	return &targetResourcesSet
}

func GetExtendResourcesUsedByPod(pod *corev1.Pod) *mapset.Set {
	extenedResourceSetUsedByPod := mapset.NewSet()
	targetResourcesSet := GetTargetResourcesSet()

	for _, container := range pod.Spec.Containers {
		for resourceName := range container.Resources.Requests {
			if (*targetResourcesSet).Contains(string(resourceName)) {
				extenedResourceSetUsedByPod.Add(string(resourceName))
			}
		}
	}

	for _, container := range pod.Spec.InitContainers {
		for resourceName := range container.Resources.Requests {
			if (*targetResourcesSet).Contains(string(resourceName)) {
				extenedResourceSetUsedByPod.Add(string(resourceName))
			}
		}
	}

	return &extenedResourceSetUsedByPod
}

func GetExtendResourceTolerationsUsedByPod(pod *corev1.Pod) *mapset.Set {
	extenedResourceTolerationsSetUsedByPod := mapset.NewSet()
	targetResourcesSet := GetTargetResourcesSet()

	for _, toleration := range pod.Spec.Tolerations {
		if (*targetResourcesSet).Contains(toleration.Key) {
			extenedResourceTolerationsSetUsedByPod.Add(toleration.Key)
		}
	}

	return &extenedResourceTolerationsSetUsedByPod
}

func GetExtendResourceTolerationsToAdd(pod *corev1.Pod) *mapset.Set {
	newTolerations := GetExtendResourcesUsedByPod(pod)
	alreadySetExtendResourceTolerations := GetExtendResourceTolerationsUsedByPod(pod)

	for toleration := range (*newTolerations).Iter() {
		if (*alreadySetExtendResourceTolerations).Contains(toleration) {
			(*newTolerations).Remove(toleration)
		}
	}

	return newTolerations
}

func GetAdmissionWebhookServer(keyPair tls.Certificate, port int) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", HandleMutate)
	mux.HandleFunc("/validate", HandleValidate)

	webhookServer := &http.Server{
		Addr:      fmt.Sprintf(":%d", port),
		Handler:   mux,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{keyPair}},
	}

	return webhookServer
}
