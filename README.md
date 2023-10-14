# Kubernetes Ingress Mutator
- [Kubernetes Ingress Mutator](#kubernetes-ingress-mutator)
  - [Description](#description)
  - [Implementation](#implementation)
    - [Core Libraries and Modules:](#core-libraries-and-modules)
    - [Key Functions:](#key-functions)
    - [Webhook Behavior:](#webhook-behavior)
    - [Running:](#running)
  - [Conclusion](#conclusion)


## Description

This project was initiated with two primary goals in mind:

1. Explore the capabilities of Kubernetes mutating webhooks.
2. Facilitate the upgrade process of Ingress objects' API version from `extensions/v1beta1` and `networking.k8s.io/v1beta1` to `networking.k8s.io/v1`.

The main intention behind this project was to see if mutating webhooks could be used to fix unsupported legacy API versions and objects. By doing so, the Kubernetes upgrade process could be simplified. With the introduction of mutating webhooks, Kubernetes users would then have the flexibility to adjust their Kubernetes objects at their own pace.

## Implementation

### Core Libraries and Modules:

- **Kubernetes API Libraries**: Used to handle, decode, and interact with Kubernetes objects.
- **HTTP and JSON**: To serve the webhook over HTTP and encode/decode requests and responses.

### Key Functions:

1. **`serve`**: Main HTTP request handler. Reads the request body, deserializes it, and processes based on its type.
2. **`readRequestBody`**: Reads the HTTP request body.
3. **`deserializeRequestBody`**: Decodes the incoming HTTP request to determine its Kubernetes kind.
4. **`createResponseObject`**: Based on the decoded request, forms the appropriate response. The main logic is applied if the request is of kind `AdmissionReview`, leading to potential modifications to the Ingress object.
5. **`sendResponse`**: Encodes and sends the response back to the Kubernetes API server.
6. **`handleError`**: Common function to handle and log errors.
7. **`mutateIngress`**: Core mutation function. Checks the provided Ingress object's annotations and, if applicable, modifies the backend service configuration.

### Webhook Behavior:

- The webhook listens on port 8443 at the `/rodsmutator` endpoint.
- When an Ingress object is created or modified, and the annotations `mutate/service-name` and `mutate/service-port` are provided, the webhook modifies the Ingress object to target the service specified by those annotations.
- If the annotations are missing or incomplete, an error is returned.
- The mutation ensures that the backend service details are replaced with the ones specified in the annotations and sets the `pathType` to `Prefix`.

### Running:

The webhook server uses TLS for secure communication. The paths to the TLS certificate and private key are hardcoded as `./tls.crt` and `./tls.key` respectively. The server starts by calling the `main` function, setting up an HTTP server, and listening for incoming requests on port `8443`.

## Conclusion

One of the key takeaways from this project is the realization that, for the request to successfully reach the mutating webhook service, a valid API version and object compatible with the Kubernetes version in use are essential. This realization led us to use annotations as a solution, minimizing the changes users would need to make to their Ingress objects. This approach helps users adapt their Ingress objects with minimal effort, making the transition smoother.
