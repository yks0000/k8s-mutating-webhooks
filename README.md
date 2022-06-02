# K8S Mutating Webhook

[Link to tutorial.](https://www.linkedin.com/pulse/extending-k8s-mutating-webhooks-yogesh-sharma)

## What are Admission Webhooks.

Read [kubernetes official documentation](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) for more details about admission webhooks.

Steps marked `OPTIONAL STEP` are mainly for users using Windows host machine for development. These steps are optional for other platform users.

## Environment

1. [Minikube](https://minikube.sigs.k8s.io/docs/start/) for development k8s cluster.
2. [Cert Manager](https://cert-manager.io/) for managing CA Certs
3. `OPTIONAL STEP`: Docker (debian) for building go project. As k8s cluster is using *nix environment, we want to make sure to have similar development environment.
4. Source code is in `src` directory.
5. All K8S config required for this tutorial is available in `configs` directory


## Installing Minikube K8S Cluster

*Note: My local machine is macOS based. As per docker [official documentation](https://docs.docker.com/desktop/mac/networking/), we can access service on host machine from container using `host.docker.internal`. Hence, we need to make sure, we add this domain to k8s cluster certificate.*

1. Please refer here for [installing minikube.](https://minikube.sigs.k8s.io/docs/start/)
2. Start minikube server.
   1. `OPTIONAL STEP`: Start with api-server name `host.docker.internal`. This tell minikube to add additional hostname as SAN to cert. Windows users can also use host network to access host services from containers.

        ```bash  
         minikube start --apiserver-names=host.docker.internal  
        ```
   2. If `OPTIONAL STEP` steps is not followed, then run

      ```bash  
      minikube start  
      ```  
      
3. `OPTIONAL STEP`: Copy `~/.kube/config` to `kube` directory at the root of this project and update `server` as `https://host.docker.internal`. Do not change the port number.

4. For Webhook, we need to have a CA which can sign certificates for TLS. We are using cert-manager for this. Alternatively you can also use Cloudflare [CFSSL](https://github.com/cloudflare/cfssl) but require lots of manual configuration. **cert-manager is highly recommended.**

   1. Run from container if `OPTIONAL STEP` steps were followed, else you can directly run from host machine. ***Access to k8s cluster is required.***

      ```bash  
        kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.0/cert-manager.yaml 
        kubectl get pods -n cert-manager  # Make sure all containers are running.  
      ```  

## Configuring Dockerfile

We will use this for building our app as well as for hosting webhook code.

1. Change directory to `src`
2. Create a `Dockerfile`

```dockerfile  
FROM golang:1.17-alpine as dev-env  
  
WORKDIR /app  
RUN apk add --no-cache curl && \  
 curl -LO https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/linux/amd64/kubectl && \ 
 chmod +x ./kubectl && \ 
 mv ./kubectl /usr/local/bin/kubectl  
```  

3. `OPTIONAL STEP` Docker for development.

   ```bash  
   docker build . -t webhook
   ```  

_**Please make sure to replace image name with your preferred name.**_


## Start a Dev Container

This section is `OPTIONAL STEP`

Start a `dev` container with Volume.

From `src` directory, run:

```bash  
docker run -it --rm -p 80:80 -p 8443:8443 -v ${PWD}/../kube/:/root/.kube/ -v ${PWD}:/app -v /Users/`whoami`/.minikube:/Users/`whoami`/.minikube webhook sh  
```  

1. **${PWD}/../kube/:/root/.kube/**: This contains updated kubeconfig file with cluster domain host.docker.internal
2. **${PWD}:/app**: Mount current `src` directory to container. Used for building app.
3. **/Users/\`whoami\`/.minikube:/Users/\`whoami\`/.minikube**: Contains all certificate which are required to connect to k8s cluster from dev container.


Note: All `go build` needs to be executed inside dev containers

## Dev environment verification

Note: Windows users should run this from docker dev container.

1. Let's define our main module and a web server inside `src` directory.

   ```bash  
   go mod init sample-mutating-webhook
   ```  

2. Create a `main.go` inside `src` with package `main`. Let's add a minimal webserver code to it

   ```go  
   package main  
     
   import (  
    "log" 
    "net/http"
   )  
     
   func main() {  
    http.HandleFunc("/", HandleRoot) 
    http.HandleFunc("/mutate", HandleMutate) 
    log.Fatal(http.ListenAndServe(":80", nil))
   }  
     
   func HandleRoot(w http.ResponseWriter, r *http.Request){  
    w.Write([]byte("HandleRoot!"))
   }  
     
   func HandleMutate(w http.ResponseWriter, r *http.Request){  
    w.Write([]byte("HandleMutate!"))
   }  
   ```  

Build:

   ```bash  
   export CGO_ENABLED=0 go build -o webhook
   ./webhook  
   ```  

Verify, if you can access `http://localhost/mutate` from Host machine browser.

Also, verify if you can access k8s cluster running on host machine

```bash  
$ kubectl get nodes
```  


## Webhook Development

1. As we will receive webhook events from kubernetes, we need to translate those requests to an understandable format such as Objects or Struct. For this we need to deserialize them using K8S serializer.

```go  
// imports added:  
  
"k8s.io/apimachinery/pkg/runtime"  
"k8s.io/apimachinery/pkg/runtime/serializer"  
  
// code:  
  
var (  
 universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
 )  
```  

2. Now to access K8S, we need to have kube config. We will use k8s config [GetConfigOrDie](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client/config#GetConfigOrDie) function.

```go  
// As global variable  
  
k8sConfig *rest.Config  
k8sClientSet *kubernetes.Clientset
  
// Inside main:  
  
k8sConfig = config.GetConfigOrDie()  
clientSet, err := kubernetes.NewForConfig(k8sConfig)  
```  


3. To test if configs are working, we added a test function inside `podscount.go`

For `podscount` to work, we also need to have a k8s client. Let's define that first in `main.go`

```go  
k8sClientSet = createClientSet()
```  

Build the app and test it.

```bash  
# ./webhook  
Total pod running in cluster: 12  
```  


5. We will also provide a way to override server port, tls location.

```go  
// ServerParameters : we need to enable a TLS endpoint  
// Let's take some parameters where we can set the path to the TLS certificate and port number to run on.  
type ServerParameters struct {  
 port           int    // webhook server port 
 certFile       string // path to the x509 certificate for https keyFile        
 string // path to the x509 private key matching `CertFile`
 }  
  
serverParameters ServerParameters  
  
  
// Inside main:  
  
flag.IntVar(&serverParameters.port, "port", 8443, "Webhook server port.")  
flag.StringVar(&serverParameters.certFile, "tlsCertFile", "/etc/webhook/certs/tls.crt", "File containing the x509 Certificate for HTTPS.")  
flag.StringVar(&serverParameters.keyFile, "tlsKeyFile", "/etc/webhook/certs/tls.key", "File containing the x509 private key to --tlsCertFile.")  
flag.Parse()  
```  

By Default, TLS files `/etc/webhook/certs/tls.crt` and `/etc/webhook/certs/tls.key` are injected using secret `sidecar-injector-certs` created using cert-manager.

6. Change Listener to TLS.

Change from

```go  
log.Fatal(http.ListenAndServe(":80", nil))  
```  

to

```go  
log.Fatal(http.ListenAndServeTLS(":" + strconv.Itoa(serverParameters.port), serverParameters.certFile, serverParameters.keyFile, nil))  
```  

7. Kubernetes sends us an AdmissionReview and expects an AdmissionResponse back. Lets us write logic to get AdmissionReview Request, pass it to universal decoder and and use it inside `HandleMutate`.

```go  
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

// inside HandleMutate  
  
// func getAdmissionReviewRequest, grab body from request, define AdmissionReview
// and use universalDeserializer to decode body to admissionReviewReq

admissionReviewReq := getAdmissionReviewRequest(w, r)
```  

8. We now need to capture Pod object from the admission request

```go  
var pod v1.Pod  
  
err = json.Unmarshal(admissionReviewReq.Request.Object.Raw, &pod)  
  
if err != nil {  
 _ = fmt.Errorf("could not unmarshal pod on admission request: %v", err)
 }  
```  

9. To perform a mutation on the object before the Kubernetes API sees the object, we need to apply a patch to the operation

```go  
var sideCarConfig *Config  
sideCarConfig = getNginxSideCarConfig()  
patches, _ := createPatch(pod, sideCarConfig)  
  
// getNginxSideCarConfig: Return Config object which contains SideCar Container and Volume Information  
// This inturn calls getPodVolumes and generateNginxSideCarConfig  
  
```  

10. Add patchBytes to the admission response

```go  
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
```  
11. Submit the response

```go  
bytes, err := json.Marshal(&admissionReviewResponse)  
if err != nil {  
   fmt.Errorf("marshaling response: %v", err)  
}  
  
w.Write(bytes)  
```  

12. Build the app

## Publish Changes to Docker Hub

We now need to publish changes to docker hub so that it can be downloaded in k8s cluster.

Change `Dockerfile` as below:

```dockerfile  
FROM golang:1.17-alpine as dev-env

WORKDIR /app
RUN apk add --no-cache curl && \
    curl -LO https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/linux/amd64/kubectl && \
    chmod +x ./kubectl && \
    mv ./kubectl /usr/local/bin/kubectl

FROM dev-env as build-env
COPY go.mod /go.sum /app/
RUN go mod download

COPY . /app/

RUN CGO_ENABLED=0 go build -o /webhook

FROM alpine:3.10 as runtime

COPY --from=build-env /webhook /usr/local/bin/webhook
RUN chmod +x /usr/local/bin/webhook

ENTRYPOINT ["webhook"]
```  

Build and Deploy

```bash  
docker build . -t yks0000/sample-mutating-webhook:v2
docker push yks0000/sample-mutating-webhook:v2
```  
  
Change name of image accordingly.  
  
## Creating K8S Resources  
  
1. Create Certificate for Webhook  
  
```bash  
$ kubectl apply -f configs/certs.yaml
issuer.cert-manager.io/selfsigned-issuer unchanged
certificate.cert-manager.io/sidecar-injector-certs unchanged
```  
  
2. Deploy RBAC  
  
```bash  
$ kubectl apply -f configs/rbac.yaml
serviceaccount/sample-mutating-webhook created
clusterrole.rbac.authorization.k8s.io/sample-mutating-webhook created
clusterrolebinding.rbac.authorization.k8s.io/sample-mutating-webhook created  
```  

3. Create Deployment

Make sure you update image to `yks0000/sample-mutating-webhook:v1`

```bash  
$ kubectl apply -f configs/deployment.yaml
deployment.apps/sample-mutating-webhook created
```  
  
Verify Pods:  
  
```bash  
$ kubectl -n default get pods | grep sample-mutating-webhook
sample-mutating-webhook-5d8666ffc7-4ljdh 1/1     Running   0          39s
```  
  
Check Logs of pod, should emit log showing total number of pods  
  
```bash  
$ kubectl logs sample-mutating-webhook-5d8666ffc7-4ljdh
Total pod running in cluster: 13
```  
  
4. Deploy Service  
  
```bash  
$ kubectl apply -f configs/service.yaml
service/sample-mutating-webhook created  
```  

5. Deploy Webhook

Make sure, you have the following annotation

```yaml  
annotations:  
 cert-manager.io/inject-ca-from: default/sidecar-injector-certs  
```  

In `default/sidecar-injector-certs`, `default` is namespace and `sidecar-injector-certs` is name of certificate that we created using `certs.yaml`

```bash  
$ kubectl apply -f configs/webhook.yaml
mutatingwebhookconfiguration.admissionregistration.k8s.io/sample-mutating-webhook created
```  
  
## Test Mutation  
  
In our example, we are adding a label `nginx-sidecar` to pod definition and injecting `nginx` container as sidecar before API server sent it to controller to schedule it.  
  
As we also added `objectSelector` to `webhook.yaml`, we need to make sure `inject-nginx-sidecar: "true"` label is added to pod definition, otherwise our mutating webhook will ignore the request.  
  
```bash  
$ kubectl apply -f configs/example-pod.yaml
```  
  
```bash  
$ kubectl get pods --show-labels | grep example-pod
example-pod                                2/2     Running   0          66m   inject-nginx-sidecar=true,nginx-sidecar=applied-from-mutating-webhook
```  
  
We can see here that `nginx-sidecar=applied-from-mutating-webhook` is added to Pod spec by Mutating webhook though it was not part of spec initially.

Also, we can see that the container count is 2, whereas in `example-pod.yaml` we only have 1 container. The other container is `nginx` container which is injected by mutating webhook written by us.

 
To access the service from browser, lets expose the port. We will expose port 443 to make sure, request is handled by injected Nginx sidecar which is doing SSL termination.

```bash
kubectl expose pod example-pod --type=LoadBalancer --port=443
```

As we are using minikube, we need to make sure we enable tunnel

```bash
minikube tunnel
```

```bash
$ curl https://127.0.0.1 -k
GET / HTTP/1.1
Host: 127.0.0.1
User-Agent: curl/7.79.1
Accept: */*
Connection: close
Time: 2022-06-01 07:35:21.4040591 +0000 UTC m=+4195.517198901
X-Forwarded-For: 172.17.0.1
```

### Verifying Logs

We are using `stern` for streaming multi-container logs

```bash
$ stern example-pod
+ example-pod › rest-api
+ example-pod › nginx-webserver
example-pod rest-api 2022/06/01 07:36:05 Echoing back request made to / to client (127.0.0.1:44724)
example-pod nginx-webserver 172.17.0.1 - - [01/Jun/2022:07:36:05 +0000] "GET / HTTP/1.1" 200 184 "-" "curl/7.79.1"
```

We can see that there are two log lines, one from `rest-api` and other from `nginx-webserver` container.

  
## Good to Know  
  
If you wish to recreate pods from Deployment, you can just delete them. **Not advisable in prod environment**  
  
```bash  
kubectl delete pods --all
```
