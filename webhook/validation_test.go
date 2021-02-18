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

const (
	succeed = "\u2713"
	failed  = "\u2717"
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

	// Set target gpu resources names
	var targetResources ArrayFlags
	nvidia := "nvidia.com/gpu"
	amd := "amd.com/gpu"
	targetResources.Set(nvidia)
	targetResources.Set(amd)
	SetTargetResourcesSet(targetResources)

	gpuPodWithGpuTolerations1 := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceName(nvidia): *resource.NewQuantity(1, resource.DecimalSI),
						},
					},
				},
			},
			Tolerations: []corev1.Toleration{
				{
					Key:      nvidia,
					Operator: "NoSchedule",
				},
				{
					Key:      nvidia,
					Operator: "NoExecute",
				},
			},
		},
	}
	gpuPodWithGpuTolerations2 := corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceName(amd): *resource.NewQuantity(1, resource.DecimalSI),
						},
					},
				},
			},
			Tolerations: []corev1.Toleration{
				{
					Key:      amd,
					Operator: "NoSchedule",
				},
				{
					Key:      amd,
					Operator: "NoExecute",
				},
			},
		},
	}
	gpuPodWithGpuTolerations3 := corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceName(nvidia): *resource.NewQuantity(1, resource.DecimalSI),
						},
					},
				},
			},
			Tolerations: []corev1.Toleration{
				{
					Key:      amd,
					Operator: "NoSchedule",
				},
				{
					Key:      amd,
					Operator: "NoExecute",
				},
			},
		},
	}
	nonGpuPodWithGpuTolerations1 := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: *resource.NewQuantity(1, resource.DecimalSI),
						},
					},
				},
			},
			Tolerations: []corev1.Toleration{
				{
					Key:      nvidia,
					Operator: "NoSchedule",
				},
				{
					Key:      nvidia,
					Operator: "NoExecute",
				},
			},
		},
	}
	nonGpuPodWithGpuTolerations2 := corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: *resource.NewQuantity(1, resource.DecimalSI),
						},
					},
				},
			},
			Tolerations: []corev1.Toleration{
				{
					Key:      amd,
					Operator: "NoSchedule",
				},
				{
					Key:      amd,
					Operator: "NoExecute",
				},
			},
		},
	}

	cases := []struct {
		description string
		in          admissionv1.AdmissionReview
		want        admissionv1.AdmissionReview
	}{
		{
			description: "A pod with Nvidia GPU which has tolerations",
			in: admissionv1.AdmissionReview{
				TypeMeta: metav1.TypeMeta{Kind: "pods", APIVersion: "v1"},
				Request:  &admissionv1.AdmissionRequest{UID: uid, Resource: podResource, Object: runtime.RawExtension{Raw: marshal(gpuPodWithGpuTolerations1)}},
			},
			want: admissionv1.AdmissionReview{Response: &admissionv1.AdmissionResponse{UID: uid, Allowed: true}},
		},
		{
			description: "A pod with AMD GPU which has tolerations",
			in: admissionv1.AdmissionReview{
				TypeMeta: metav1.TypeMeta{Kind: "pods", APIVersion: "v1"},
				Request:  &admissionv1.AdmissionRequest{UID: uid, Resource: podResource, Object: runtime.RawExtension{Raw: marshal(gpuPodWithGpuTolerations2)}},
			},
			want: admissionv1.AdmissionReview{Response: &admissionv1.AdmissionResponse{UID: uid, Allowed: true}},
		},
		{
			description: "A pod with Nvidia GPU which has AMD tolerations",
			in: admissionv1.AdmissionReview{
				TypeMeta: metav1.TypeMeta{Kind: "pods", APIVersion: "v1"},
				Request:  &admissionv1.AdmissionRequest{UID: uid, Resource: podResource, Object: runtime.RawExtension{Raw: marshal(gpuPodWithGpuTolerations3)}},
			},
			want: admissionv1.AdmissionReview{Response: &admissionv1.AdmissionResponse{UID: uid, Allowed: false}},
		},
		{
			description: "A pod with no extended resources which has Nvidia tolerations",
			in: admissionv1.AdmissionReview{
				TypeMeta: metav1.TypeMeta{Kind: "pods", APIVersion: "v1"},
				Request:  &admissionv1.AdmissionRequest{UID: uid, Resource: podResource, Object: runtime.RawExtension{Raw: marshal(nonGpuPodWithGpuTolerations1)}},
			},
			want: admissionv1.AdmissionReview{Response: &admissionv1.AdmissionResponse{UID: uid, Allowed: false}},
		},
		{
			description: "A pod with no extended resources which has AMD tolerations",
			in: admissionv1.AdmissionReview{
				TypeMeta: metav1.TypeMeta{Kind: "pods", APIVersion: "v1"},
				Request:  &admissionv1.AdmissionRequest{UID: uid, Resource: podResource, Object: runtime.RawExtension{Raw: marshal(nonGpuPodWithGpuTolerations2)}},
			},
			want: admissionv1.AdmissionReview{Response: &admissionv1.AdmissionResponse{UID: uid, Allowed: false}},
		},
	}

	for _, c := range cases {
		t.Logf("\tTest: %v", c.description)
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

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(HandleValidate)
		handler.ServeHTTP(rr, req)

		var admissionReview admissionv1.AdmissionReview
		_, _, err = universalDeserializer.Decode([]byte(rr.Body.String()), nil, &admissionReview)
		if admissionReview.Response.Allowed != c.want.Response.Allowed {
			t.Errorf("\t%s\thandler returned unexpected result: got %v want %v", failed,
				admissionReview.Response.Allowed, c.want.Response.Allowed)
		} else {
			t.Logf("\t%s\thandler returned: %v.", succeed, admissionReview.Response.Allowed)
		}
	}
}
