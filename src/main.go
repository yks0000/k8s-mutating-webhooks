package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/json"
	_ "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"strconv"
)

// ServerParameters : we need to enable a TLS endpoint
// Let's take some parameters where we can set the path to the TLS certificate and port number to run on.
type ServerParameters struct {
	port           int    // webhook server port
	certFile       string // path to the x509 certificate for https
	keyFile        string // path to the x509 private key matching `CertFile`
}

// To perform a simple mutation on the object before the Kubernetes API sees the object, we can apply a patch to the operation. RFC6902
type patchOperation struct {
	Op    string      `json:"op"`  // Operation
	Path  string      `json:"path"` // Path
	Value interface{} `json:"value,omitempty"`
}

// Config To perform patching to Pod definition
type Config struct {
	Containers []corev1.Container `yaml:"containers"`
	Volumes    []corev1.Volume    `yaml:"volumes"`
}

var (
	universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
	k8sConfig *rest.Config
	k8sClientSet *kubernetes.Clientset
	serverParameters ServerParameters
)


func main() {
	flag.IntVar(&serverParameters.port, "port", 8443, "Webhook server port.")
	flag.StringVar(&serverParameters.certFile, "tlsCertFile", "/etc/webhook/certs/tls.crt", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&serverParameters.keyFile, "tlsKeyFile", "/etc/webhook/certs/tls.key", "File containing the x509 private key to --tlsCertFile.")
	flag.Parse()

	// Creating Client Set.
	k8sClientSet = createClientSet()

	// test if client set is working
	podsCount()

	http.HandleFunc("/", HandleRoot)
	http.HandleFunc("/mutate", HandleMutate)
	log.Fatal(http.ListenAndServeTLS(":" + strconv.Itoa(serverParameters.port), serverParameters.certFile, serverParameters.keyFile, nil))
}

func HandleRoot(w http.ResponseWriter, r *http.Request){
	_, err := w.Write([]byte("HandleRoot!"))
	if err != nil {
		return
	}
}

func getAdmissionReviewRequest(w http.ResponseWriter, r *http.Request) admissionv1.AdmissionReview {

	// Grabbing the http body received on webhook.
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err.Error())
	}

	// Required to pass to universal decoder.
	// v1beta1 also needs to be added to webhook.yaml
	var admissionReviewReq admissionv1.AdmissionReview

	if _, _, err := universalDeserializer.Decode(body, nil, &admissionReviewReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = fmt.Errorf("could not deserialize request: %v", err)
	} else if admissionReviewReq.Request == nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = errors.New("malformed admission review: request is nil")
	}
	return admissionReviewReq
}

func HandleMutate(w http.ResponseWriter, r *http.Request){
	// func getAdmissionReviewRequest, grab body from request, define AdmissionReview
	// and use universalDeserializer to decode body to admissionReviewReq
	admissionReviewReq := getAdmissionReviewRequest(w, r)

	// Debug statement to verify if universalDeserializer worked
	fmt.Printf("Type: %v \t Event: %v \t Name: %v \n",
		admissionReviewReq.Request.Kind,
		admissionReviewReq.Request.Operation,
		admissionReviewReq.Request.Name,
	)

	// We now need to capture Pod object from the admission request
	var pod corev1.Pod
	err := json.Unmarshal(admissionReviewReq.Request.Object.Raw, &pod)
	if err != nil {
		_ = fmt.Errorf("could not unmarshal pod on admission request: %v", err)
	}

	// To perform a mutation on the object before the Kubernetes API sees the object, we can apply a patch to the operation
	// Add Labels
	var sideCarConfig *Config
	sideCarConfig = getNginxSideCarConfig()
	patches, _ := createPatch(pod, sideCarConfig)

	// Once you have completed all patching, convert the patches to byte slice:
	patchBytes, err := json.Marshal(patches)
	if err != nil {
		_ = fmt.Errorf("could not marshal JSON patch: %v", err)
	}

	// Add patchBytes to the admission response
	admissionReviewResponse := admissionv1.AdmissionReview{
		Response: &admissionv1.AdmissionResponse{
			UID: admissionReviewReq.Request.UID,
			Allowed: true,
		},
	}
	admissionReviewResponse.Response.Patch = patchBytes

	// Submit the response
	bytes, err := json.Marshal(&admissionReviewResponse)
	if err != nil {
		_ = fmt.Errorf("marshaling response: %v", err)
	}

	_, err = w.Write(bytes)
	if err != nil {
		return
	}

}