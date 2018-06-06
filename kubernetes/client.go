package kubernetes

import (
	"fmt"
	"net"
	"os"

	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kube "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	kialiConfig "github.com/kiali/kiali/config"
	"github.com/kiali/kiali/log"
)

const (
	// These constants are tweaks to the k8s client I think once are set up they won't change so no need to put them on the config
	// Default QPS and Burst are quite low and those are not designed for a backend that should perform several
	// queries to build an inventory of entities from a k8s backend.
	// Other k8s clients have increased these values to a similar values.
	k8sQPS   = 100
	k8sBurst = 200
)

var (
	emptyListOptions = meta_v1.ListOptions{}
	emptyGetOptions  = meta_v1.GetOptions{}
)

// IstioClientInterface for mocks (only mocked function are necessary here)
type IstioClientInterface interface {
	GetNamespaces() (*v1.NamespaceList, error)
	GetService(namespace string, serviceName string) (*v1.Service, error)
	GetServices(namespaceName string) (*ServiceList, error)
	GetServiceDetails(namespace string, serviceName string) (*ServiceDetails, error)
	GetPods(namespace string, labelsSet labels.Set) (*v1.PodList, error)
	GetNamespacePods(namespace string) (*v1.PodList, error)
	GetServicePods(namespace string, serviceName string, serviceVersion string) (*v1.PodList, error)
	GetIstioDetails(namespace string, serviceName string) (*IstioDetails, error)
	GetRouteRules(namespace string, serviceName string) ([]IstioObject, error)
	GetRouteRule(namespace string, routerule string) (IstioObject, error)
	CreateRouteRule(namespace string, routerule IstioObject) (IstioObject, error)
	GetDestinationPolicies(namespace string, serviceName string) ([]IstioObject, error)
	GetDestinationPolicy(namespace string, destinationpolicy string) (IstioObject, error)
	GetVirtualServices(namespace string, serviceName string) ([]IstioObject, error)
	GetVirtualService(namespace string, virtualservice string) (IstioObject, error)
	GetDestinationRules(namespace string, serviceName string) ([]IstioObject, error)
	GetDestinationRule(namespace string, destinationrule string) (IstioObject, error)
	GetIstioRules(namespace string) (*IstioRules, error)
	GetIstioRuleDetails(namespace string, istiorule string) (*IstioRuleDetails, error)
}

// IstioClient is the client struct for Kubernetes and Istio APIs
// It hides the way it queries each API
type IstioClient struct {
	IstioClientInterface
	k8s                *kube.Clientset
	istioConfigApi     *rest.RESTClient
	istioNetworkingApi *rest.RESTClient
}

// ConfigClient return a client with the correct configuration
// Returns configuration if Kiali is in Cluster when InCluster is true
// Returns configuration if Kiali is not int Cluster when InCluster is false
// It returns an error on any problem
func ConfigClient() (*rest.Config, error) {
	if kialiConfig.Get().InCluster {
		return rest.InClusterConfig()
	}
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return nil, fmt.Errorf("unable to load in-cluster configuration, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined")
	}

	return &rest.Config{
		// TODO: switch to using cluster DNS.
		Host: "http://" + net.JoinHostPort(host, port),
	}, nil
}

// NewClient creates a new client to the Kubernetes and Istio APIs.
// It takes the assumption that Istio is deployed into the cluster.
// It hides the access to Kubernetes/Openshift credentials.
// It hides the low level use of the API of Kubernetes and Istio, it should be considered as an implementation detail.
// It returns an error on any problem.
func NewClient() (*IstioClient, error) {
	client := IstioClient{}
	config, err := ConfigClient()

	if err != nil {
		return nil, err
	}

	config.QPS = k8sQPS
	config.Burst = k8sBurst

	k8s, err := kube.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	client.k8s = k8s

	// Istio is a CRD extension of Kubernetes API, so any custom type should be registered here.
	// KnownTypes registers the Istio objects we use, as soon as we get more info we will increase the number of types.
	types := runtime.NewScheme()
	schemeBuilder := runtime.NewSchemeBuilder(
		func(scheme *runtime.Scheme) error {
			for _, kind := range istioKnownTypes {
				scheme.AddKnownTypes(*kind.groupVersion, kind.object, kind.collection)
			}
			meta_v1.AddToGroupVersion(scheme, istioConfigGroupVersion)
			meta_v1.AddToGroupVersion(scheme, istioNetworkingGroupVersion)
			return nil
		})

	err = schemeBuilder.AddToScheme(types)
	if err != nil {
		return nil, err
	}

	// Istio needs another type as it queries a different K8S API.
	istioConfig := rest.Config{
		Host:    config.Host,
		APIPath: "/apis",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &istioConfigGroupVersion,
			NegotiatedSerializer: serializer.DirectCodecFactory{CodecFactory: serializer.NewCodecFactory(types)},
			ContentType:          runtime.ContentTypeJSON,
		},
		BearerToken:     config.BearerToken,
		TLSClientConfig: config.TLSClientConfig,
		QPS:             config.QPS,
		Burst:           config.Burst,
	}

	istioConfigApi, err := rest.RESTClientFor(&istioConfig)
	client.istioConfigApi = istioConfigApi
	if err != nil {
		return nil, err
	}

	istioNetworking := rest.Config{
		Host:    config.Host,
		APIPath: "/apis",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &istioNetworkingGroupVersion,
			NegotiatedSerializer: serializer.DirectCodecFactory{CodecFactory: serializer.NewCodecFactory(types)},
			ContentType:          runtime.ContentTypeJSON,
		},
		BearerToken:     config.BearerToken,
		TLSClientConfig: config.TLSClientConfig,
		QPS:             config.QPS,
		Burst:           config.Burst,
	}

	istioNetworkingApi, err := rest.RESTClientFor(&istioNetworking)
	client.istioNetworkingApi = istioNetworkingApi
	if err != nil {
		return nil, err
	}

	return &client, nil
}

// FilterDeploymentsForService returns a subpart of deployments list filtered according to pods labels.
func FilterDeploymentsForService(s *v1.Service, allPods *v1.PodList, allDepls *v1beta1.DeploymentList) []v1beta1.Deployment {
	if s == nil || allDepls == nil || allPods == nil {
		return nil
	}
	serviceSelector := labels.Set(s.Spec.Selector).AsSelector()
	pods := filterPodsForService(serviceSelector, allPods)

	var deployments []v1beta1.Deployment
	for _, d := range allDepls.Items {
		depSelector, err := meta_v1.LabelSelectorAsSelector(d.Spec.Selector)
		if err != nil {
			log.Errorf("Invalid label selector: %v", err)
		}
		added := false
		// If it matches any of the pods, then it's "part" of the service
		for _, pod := range pods {
			// If a deployment with an empty selector creeps in, it should match nothing, not everything.
			if !depSelector.Empty() && depSelector.Matches(labels.Set(pod.ObjectMeta.Labels)) {
				deployments = append(deployments, d)
				added = true
				break
			}
		}
		if !added {
			// Maybe there's no pod (yet) for a deployment, but it still "belongs" to that service
			// We can try to guess that by matching service selector with deployment labels and assume they would match.
			// This is of course not guaranteed.
			if !serviceSelector.Empty() && serviceSelector.Matches(labels.Set(d.ObjectMeta.Labels)) {
				deployments = append(deployments, d)
			}
		}
	}
	return deployments
}

func filterPodsForService(selector labels.Selector, allPods *v1.PodList) []v1.Pod {
	var pods []v1.Pod
	for _, pod := range allPods.Items {
		if selector.Matches(labels.Set(pod.ObjectMeta.Labels)) {
			pods = append(pods, pod)
		}
	}
	return pods
}
