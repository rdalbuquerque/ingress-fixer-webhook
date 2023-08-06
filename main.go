package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
)

func init() {
	v1.AddToScheme(runtimeScheme)
	networkingv1.AddToScheme(runtimeScheme)
}

func serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("contentType=%s, expect application/json", contentType)
		http.Error(w, "Invalid content type, expect application/json", http.StatusBadRequest)
		return
	}

	klog.V(2).Info(fmt.Sprintf("handling request: %s", body))

	deserializer := codecs.UniversalDeserializer()
	obj, gvk, err := deserializer.Decode(body, nil, nil)
	if err != nil {
		msg := fmt.Sprintf("Request could not be decoded: %v", err)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	var responseObj runtime.Object
	switch *gvk {
	case v1.SchemeGroupVersion.WithKind("AdmissionReview"):
		requestedAdmissionReview, ok := obj.(*v1.AdmissionReview)
		if !ok {
			msg := fmt.Sprintf("Expected v1.AdmissionReview but got: %T", obj)
			klog.Errorf(msg)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		responseAdmissionReview := &v1.AdmissionReview{}
		responseAdmissionReview.SetGroupVersionKind(*gvk)
		if responseAdmissionReview.Response = mutateIngress(*requestedAdmissionReview); responseAdmissionReview.Response == nil {
			msg := fmt.Sprintf("Response was nil: %v", gvk)
			klog.Error(msg)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
		responseObj = responseAdmissionReview
	default:
		msg := fmt.Sprintf("Unsupported group version kind: %v", gvk)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	klog.V(2).Info(fmt.Sprintf("sending response: %v", responseObj))
	respBytes, err := json.Marshal(responseObj)
	if err != nil {
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func mutateIngress(ar v1.AdmissionReview) *v1.AdmissionResponse {
	patch := `[
		{"op":"replace","path":"/spec/rules/0/http/paths/0/backend","value":{"service":{"name": "test", "port": {"number": 80}}}}
	]`
	klog.Info("mutating ingress")
	ingressResource := metav1.GroupVersionResource{Resource: "ingresses"}
	if ar.Request.Resource.Resource != ingressResource.Resource {
		klog.Errorf("expect resource to be %s, but got %s", ingressResource.Resource, ar.Request.Resource.Resource)
		return nil
	}

	raw := ar.Request.Object.Raw
	ingress := networkingv1.Ingress{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(raw, nil, &ingress); err != nil {
		klog.Error(err)
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}
	reviewResponse := v1.AdmissionResponse{}
	reviewResponse.Allowed = true
	reviewResponse.Patch = []byte(patch)
	pt := v1.PatchTypeJSONPatch
	reviewResponse.PatchType = &pt
	return &reviewResponse
}

func main() {
	config := Config{
		CertFile: "./tls.crt",
		KeyFile:  "./tls.key",
	}

	http.HandleFunc("/rodsmutator", serve)
	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", 8443),
		TLSConfig: configTLS(config),
	}
	klog.Info("Serving on :8443")
	err := server.ListenAndServeTLS("", "")
	if err != nil {
		panic(err)
	}
}
