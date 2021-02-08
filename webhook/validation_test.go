package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestValidate(t *testing.T) {
	uid := types.UID("12D3FG")
	request := v1beta1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{Kind: "pods", APIVersion: "v1"},
		Request:  &v1beta1.AdmissionRequest{UID: uid, Resource: podResource},
		Response: &v1beta1.AdmissionResponse{},
	}

	pbytes, err := json.Marshal(request)
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
	expected := `{"response":{"uid":"` + string(uid) + `","allowed":true}}`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}
