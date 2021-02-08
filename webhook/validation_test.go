package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func marshal(p corev1.Pod) []byte {
	b, err := json.Marshal(p)
	if err != nil {
		return nil
	}

	return b
}

func TestValidate(t *testing.T) {
	uid := types.UID("12D3FG")
	gpuResourceName := "nvidia.com/gpu" // TODO: gpu device list 관리 필요
	gpuPod := []corev1.Pod{
		{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceName(gpuResourceName): *resource.NewQuantity(1, resource.DecimalSI),
							},
						},
					},
				},
				Tolerations: []corev1.Toleration{
					{
						Key:      gpuResourceName,
						Operator: "NoSchedule",
					},
					{
						Key:      gpuResourceName,
						Operator: "NoExecute",
					},
				},
			},
		},
		{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceName(gpuResourceName): *resource.NewQuantity(1, resource.DecimalSI),
							},
						},
					},
				},
			},
		},
		{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU: *resource.NewQuantity(1, resource.DecimalSI),
							},
						},
					},
				},
				Tolerations: []corev1.Toleration{
					{
						Key:      gpuResourceName,
						Operator: "NoSchedule",
					},
					{
						Key:      gpuResourceName,
						Operator: "NoExecute",
					},
				},
			},
		},
	}

	cases := []struct {
		in   admissionv1.AdmissionReview
		want string
	}{
		{
			in: admissionv1.AdmissionReview{
				TypeMeta: metav1.TypeMeta{Kind: "pods", APIVersion: "v1"},
				Request:  &admissionv1.AdmissionRequest{UID: uid, Resource: podResource, Object: runtime.RawExtension{Raw: marshal(gpuPod[0])}},
			},
			want: `{"response":{"uid":"` + string(uid) + `","allowed":true}}`,
		},
		{
			in: admissionv1.AdmissionReview{
				TypeMeta: metav1.TypeMeta{Kind: "pods", APIVersion: "v1"},
				Request:  &admissionv1.AdmissionRequest{UID: uid, Resource: podResource, Object: runtime.RawExtension{Raw: marshal(gpuPod[1])}},
			},
			want: `{"response":{"uid":"` + string(uid) + `","allowed":false,"status":{"metadata":{},"message":"Forbidden Toleration Usage: Empty toleration"}}}`,
		},
		{
			in: admissionv1.AdmissionReview{
				TypeMeta: metav1.TypeMeta{Kind: "pods", APIVersion: "v1"},
				Request:  &admissionv1.AdmissionRequest{UID: uid, Resource: podResource, Object: runtime.RawExtension{Raw: marshal(gpuPod[2])}},
			},
			want: `{"response":{"uid":"` + string(uid) + `","allowed":false,"status":{"metadata":{},"message":"Forbidden Toleration Usage: Untolerated key: nvidia.com/gpu"}}}`,
		},
	}

	for _, c := range cases {
		pbytes, err := json.Marshal(c.in)
		if err != nil {
			t.Errorf("marshaling response: %v", err)
		}
		buff := bytes.NewBuffer(pbytes)

		req, err := http.NewRequest("POST", "/validate", buff)
		if err != nil {
			t.Error()
		}
		req.Header.Set("Content-Type", "application/json")

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(HandleValidate)

		// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
		// directly and pass in our Request and ResponseRecorder.
		handler.ServeHTTP(rr, req)

		// Check the status code is what we expect.
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}

		// Check the response body is what we expect.
		if rr.Body.String() != c.want {
			t.Errorf("handler returned unexpected body: got %v want %v",
				rr.Body.String(), c.want)
		}
	}
}
