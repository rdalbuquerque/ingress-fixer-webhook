package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	body, err := readRequestBody(r)
	if err != nil {
		handleError(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
		return
	}

	klog.V(2).Info(fmt.Sprintf("handling request: %s", body))

	obj, gvk, err := deserializeRequestBody(body)
	if err != nil {
		handleError(w, fmt.Sprintf("Request could not be decoded: %v", err), http.StatusBadRequest)
		return
	}

	responseObj, err := createResponseObject(obj, gvk)
	if err != nil {
		handleError(w, err.Error(), http.StatusBadRequest)
		return
	}

	sendResponse(w, responseObj)
}

func readRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, errors.New("request body is nil")
	}
	return io.ReadAll(r.Body)
}

func deserializeRequestBody(body []byte) (runtime.Object, *schema.GroupVersionKind, error) {
	deserializer := codecs.UniversalDeserializer()
	return deserializer.Decode(body, nil, nil)
}

func createResponseObject(obj runtime.Object, gvk *schema.GroupVersionKind) (runtime.Object, error) {
	switch *gvk {
	case v1.SchemeGroupVersion.WithKind("AdmissionReview"):
		requestedAdmissionReview, ok := obj.(*v1.AdmissionReview)
		if !ok {
			return nil, fmt.Errorf("expected v1.AdmissionReview but got: %T", obj)
		}

		responseAdmissionReview := &v1.AdmissionReview{}
		responseAdmissionReview.SetGroupVersionKind(*gvk)
		if responseAdmissionReview.Response = mutateIngress(*requestedAdmissionReview); responseAdmissionReview.Response.Result != nil {
			return nil, fmt.Errorf(responseAdmissionReview.Response.Result.Message)
		}

		responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
		return responseAdmissionReview, nil
	default:
		return nil, fmt.Errorf("unsupported group version kind: %v", gvk)
	}
}

func sendResponse(w http.ResponseWriter, responseObj runtime.Object) {
	klog.V(2).Info(fmt.Sprintf("sending response: %v", responseObj))

	respBytes, err := json.Marshal(responseObj)
	if err != nil {
		handleError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		handleError(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleError(w http.ResponseWriter, msg string, statusCode int) {
	klog.Error(msg)
	http.Error(w, msg, statusCode)
}

func mutateIngress(ar v1.AdmissionReview) *v1.AdmissionResponse {
	raw := ar.Request.Object.Raw
	klog.Infof("mutating object: %s", string(raw))
	ingress := extensionsv1beta1.Ingress{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(raw, nil, &ingress); err != nil {
		klog.Error(err)
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	serviceName := ingress.ObjectMeta.Annotations["mutate/service-name"]
	servicePort := ingress.ObjectMeta.Annotations["mutate/service-port"]
	if serviceName == "" || servicePort == "" {
		err := errors.New("couldn't retrive service name or port from configuration")
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	patch := fmt.Sprintf(`[
		{"op":"replace","path":"/spec/rules/0/http/paths/0/backend","value":{"service":{"name": "%s", "port": {"number": %s}}}},
		{"op":"add","path":"/spec/rules/0/http/paths/0/pathType","value":"Prefix"}
	]`, serviceName, servicePort)
	klog.Infof("mutating ingress with name %s and port %s with operations: %s", serviceName, servicePort, patch)
	ingressResource := metav1.GroupVersionResource{Resource: "ingresses"}
	if ar.Request.Resource.Resource != ingressResource.Resource {
		errMsg := fmt.Sprintf("expect resource to be %s, but got %s", ingressResource.Resource, ar.Request.Resource.Resource)
		err := errors.New(errMsg)
		klog.Error(errMsg)
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
