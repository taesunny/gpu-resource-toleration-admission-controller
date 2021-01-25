package webhook

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog"

	mapset "github.com/deckarep/golang-set"
)

const (
	controllerNameSpaceName string = "kube-system"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

type patchOps struct {
	// https://kubernetes.io/blog/2019/03/21/a-guide-to-kubernetes-admission-controllers/
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func HandleMutate(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		klog.Error("Empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	_, _, err := deserializer.Decode(body, nil, &ar)
	if err != nil {
		klog.Errorf("Can't decode body: %s", err)
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = mutate(&ar)
	}

	admissionReview := v1beta1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		klog.Errorf("Couldn't encode response: %s", err)
		http.Error(w, fmt.Sprintf("couldn't encode response: %s", err), http.StatusInternalServerError)
	}

	klog.Infof("Writing response...")

	_, err = w.Write(resp)
	if err != nil {
		klog.Errorf("Couldn't write response: %s", err)
		http.Error(w, fmt.Sprintf("couldn't write response: %s", err), http.StatusInternalServerError)
	}
}

func mutate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := ar.Request

	var pod corev1.Pod
	err := json.Unmarshal(req.Object.Raw, &pod)
	if err != nil {
		klog.Errorf("Could not unmarshal raw object: %s", err)
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	tolerationsToAdd := getTolerationsToAdd(pod)

	if (*tolerationsToAdd).Cardinality() == 0 {
		klog.Infof("No need to mutate, Pod name: %s/%s", pod.Name, pod.Namespace)

		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	patchData, err := getTolerationsPatchData(pod, tolerationsToAdd)

	if err != nil {
		klog.Errorf("Could not make patch data: %s", err)
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	klog.Infof("AdmissionResponse: patch=%s", string(patchData))
	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patchData,
		PatchType: func() *v1beta1.PatchType {
			patchType := v1beta1.PatchTypeJSONPatch
			return &patchType
		}(),
	}
}

func getTolerationsPatchData(pod corev1.Pod, tolerationsToAdd *mapset.Set) ([]byte, error) {
	var patch []patchOps

	if pod.Spec.Tolerations == nil {
		patch = append(patch, patchOps{
			Op:    "add",
			Path:  "/spec/tolerations",
			Value: getTolerationObjects(tolerationsToAdd),
		})
	} else {
		for v := range (*tolerationsToAdd).Iter() {
			if toleration, ok := v.(string); ok {
				pod.Spec.Tolerations = append(pod.Spec.Tolerations, getTolerationObject(toleration))
			}
		}

		patch = append(patch, patchOps{
			Op:    "replace",
			Path:  "/spec/tolerations",
			Value: pod.Spec.Tolerations,
		})
	}

	return json.Marshal(patch)
}

func getTolerationObject(key string) corev1.Toleration {
	var toleration corev1.Toleration

	toleration.Key = key
	toleration.Operator = corev1.TolerationOpExists
	toleration.Effect = corev1.TaintEffectNoExecute

	return toleration
}

func getTolerationObjects(tolerationsToAdd *mapset.Set) []corev1.Toleration {
	var tolerations []corev1.Toleration

	for v := range (*tolerationsToAdd).Iter() {
		if toleration, ok := v.(string); ok {
			tolerations = append(tolerations, getTolerationObject(toleration))
		}
	}

	return tolerations
}

func getTolerationsToAdd(pod corev1.Pod) *mapset.Set {
	taintsSetToAdd := mapset.NewSet()
	targetResourcesSet := GetTargetResourcesSet()

	for _, container := range pod.Spec.Containers {
		for resourceName := range container.Resources.Requests {
			if (*targetResourcesSet).Contains(string(resourceName)) {
				taintsSetToAdd.Add(string(resourceName))
			}
		}
	}

	for _, container := range pod.Spec.InitContainers {
		for resourceName := range container.Resources.Requests {
			if (*targetResourcesSet).Contains(string(resourceName)) {
				taintsSetToAdd.Add(string(resourceName))
			}
		}
	}

	return &taintsSetToAdd
}
