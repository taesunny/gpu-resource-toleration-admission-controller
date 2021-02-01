package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/klog"
	corev1 "k8s.io/api/core/v1"
	// api "k8s.io/kubernetes/pkg/apis/core"
)

func HandleValidate(w http.ResponseWriter, r *http.Request) {
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

	var admissionResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	_, _, err := Deserializer.Decode(body, nil, &ar)
	if err != nil {
		klog.Errorf("Can't decode body: %s", err)
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		// err := validate(&ar)
		var err error = nil

		if err == nil {
			admissionResponse = &v1beta1.AdmissionResponse{
				Allowed: true,
				Result: &v1.Status{
					Message: "Validated!",
				},
			}
		} else {
			admissionResponse = &v1beta1.AdmissionResponse{
				Allowed: false,
				Result: &v1.Status{
					Message: "InValid!",
				},
			}
		}
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

func validate(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {

	pod := a.GetObject().(*corev1.Pod)!!!!!!!!!!!!!!

	extendedResourcesUsedByPod := GetExtendResourceTolerationsUsedByPod(&pod)
	extenedResourceTolerationsUsedByPod := GetExtendResourceTolerationsUsedByPod(&pod)

	if !(*extenedResourceTolerationsUsedByPod).IsSubset(*extendedResourcesUsedByPod) {
		return admission.NewForbidden(a, fmt.Errorf("Forbidden Toleration Usage"))
	}

	return nil
}
