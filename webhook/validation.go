package webhook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog"
)

const (
	jsonContentType = `application/json`
)

var (
	universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
	podResource           = metav1.GroupVersionResource{Version: "v1", Resource: "pods"}
)

// HandleValidate is a wrapper around validate that adds error handling and logging
func HandleValidate(w http.ResponseWriter, r *http.Request) {
	klog.Info("Handling validation request ...")

	var writeErr error
	if bytes, err := validate(w, r); err != nil {
		klog.Errorf("Error handling validation request: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr = w.Write([]byte(err.Error()))
	} else {
		klog.Info("Validation request handled successfully")
		_, writeErr = w.Write(bytes)
	}

	if writeErr != nil {
		klog.Errorf("Could not write respose: %v", writeErr)
	}
}

// validate parses the HTTP request for an admission controller webhook. The response body
// is then returned as raw bytes.
func validate(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	// Request validation. Only handle POST requests with a body and json content type.

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil, fmt.Errorf("invalid method %s, only POST requests are allowed", r.Method)
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("could not read request body: %v", err)
	}

	if contentType := r.Header.Get("Content-Type"); contentType != jsonContentType {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("unsupported content type %s, only %s is supported", contentType, jsonContentType)
	}

	// Parse the AdmissionReview request

	var admissionReviewReq v1beta1.AdmissionReview
	if _, _, err := universalDeserializer.Decode(body, nil, &admissionReviewReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("could not deserialize request: %v", err)
	} else if admissionReviewReq.Request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, errors.New("Malformed admission review: request is nil")
	}

	// Construct the AdmissionReview response

	admissionReviewResponse := v1beta1.AdmissionReview{
		Response: &v1beta1.AdmissionResponse{
			UID: admissionReviewReq.Request.UID,
		},
	}

	// validate the gpu option
	if err = validateGpu(admissionReviewReq.Request); err != nil {
		// If the handler returned an error, incorporate the error message
		// into the response and deny the object creation.
		admissionReviewResponse.Response.Allowed = false
		admissionReviewResponse.Response.Result = &metav1.Status{
			Message: err.Error(),
		}
	} else {
		admissionReviewResponse.Response.Allowed = true
	}

	// Return the AdmissionReview with a response as JSON
	bytes, err := json.Marshal(admissionReviewResponse)
	if err != nil {
		return nil, fmt.Errorf("marshaling response: %v", err)
	}
	return bytes, nil
}

// validateGpu validates wether the given request has permission on
// using GPU device.
func validateGpu(req *v1beta1.AdmissionRequest) error {
	// This handler should only get called on Pod objects.
	// However, if different kind of object is invoked, issue a log message
	// but let the object request pass through.

	if req.Resource != podResource {
		klog.Infof("expect resource to be: %s, instead request resource: %s", podResource, req.Resource)
		return nil
	}

	// Parse the Pod object.
	raw := req.Object.Raw
	pod := corev1.Pod{}
	if _, _, err := universalDeserializer.Decode(raw, nil, &pod); err != nil {
		return fmt.Errorf("could not deserialize pod object: %v", err)
	}

	extendedResourcesUsedByPod := GetExtendResourcesUsedByPod(&pod)
	extenedResourceTolerationsUsedByPod := GetExtendResourceTolerationsUsedByPod(&pod)

	if !(*extenedResourceTolerationsUsedByPod).IsSubset(*extendedResourcesUsedByPod) {
		return fmt.Errorf("Forbidden Toleration Usage")
	}

	return nil
}
