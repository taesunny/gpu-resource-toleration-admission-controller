package webhook

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/apiserver/pkg/admission"
)

func HandleValidate(w http.ResponseWriter, r *http.Request) {

}

func validate(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {

	pod := a.GetObject().(*api.Pod)

	if pod.Spec.Tolerations != nil {
		targetResourcesSet := GetTargetResourcesSet()

		for _, toleration := range pod.Spec.Tolerations {
			if targetResourcesSet.Contains(toleration.key) {
				return fmt.Errorf("System Toleration Key:", whitelistScope)
			}
		}
	}

	return nil
}

func getExtendResourceSetUsedByPod(pod *api.Pod) {

}
