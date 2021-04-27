package lib

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// KubeHelper is a utility class for orchestrating containers using Kubernetes.
type KubeHelper struct {
	config    *rest.Config
	kube      *kubernetes.Clientset
	namespace string
	logger    *zap.SugaredLogger
}

// NewKubeHelper creates a new KubeHelper object.
func NewKubeHelper(namespace string, logger *zap.SugaredLogger) (*KubeHelper, error) {

	kubeCfgPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	kubeCfg, err := clientcmd.BuildConfigFromFlags("", kubeCfgPath)
	if err != nil {
		return nil, err
	}
	kube, err := kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		return nil, err
	}
	return &KubeHelper{
		config:    kubeCfg,
		kube:      kube,
		namespace: namespace,
		logger:    logger,
	}, nil
}

func (k *KubeHelper) EnsureConfigMap(ctx context.Context, cmapName string, cmapYAML string) error {
	cmaps := k.kube.CoreV1().ConfigMaps(k.namespace)
	cmap, err := cmaps.Get(ctx, cmapName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		var newCmap corev1.ConfigMap
		err := yaml.Unmarshal([]byte(cmapYAML), &newCmap)
		if err != nil {
			return errors.Wrap(err, "unmarshaling ConfigMap YAML")
		}
		cmap, err = cmaps.Create(ctx, &newCmap, metav1.CreateOptions{})
		if err != nil {
			return errors.Wrap(err, "creating ConfigMap")
		}
	}
	_ = cmap
	return nil
}

func (k *KubeHelper) EnsureSecret(ctx context.Context, secretName string, secretYAML string) error {
	secrets := k.kube.CoreV1().Secrets(k.namespace)
	secret, err := secrets.Get(ctx, secretName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		var newSecret corev1.Secret
		err := yaml.Unmarshal([]byte(secretYAML), &newSecret)
		if err != nil {
			return errors.Wrap(err, "unmarshaling Secret YAML")
		}
		secret, err = secrets.Create(ctx, &newSecret, metav1.CreateOptions{})
		if err != nil {
			return errors.Wrap(err, "creating Secret")
		}
	}
	_ = secret
	return nil
}

func ExtractBytesFromSecretYAML(secretYAML string, key string) ([]byte, error) {
	var secret corev1.Secret
	err := yaml.Unmarshal([]byte(secretYAML), &secret)
	if err != nil {
		return nil, err
	}
	data, _ := secret.Data[key]
	return data, nil
}

func (k *KubeHelper) EnsurePod(ctx context.Context, podName string, podYAML string) error {
	pods := k.kube.CoreV1().Pods(k.namespace)
	pod, err := pods.Get(ctx, podName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		var newPod corev1.Pod
		err := yaml.Unmarshal([]byte(podYAML), &newPod)
		if err != nil {
			return errors.Wrap(err, "unmarshaling Pod YAML")
		}
		pod, err = pods.Create(ctx, &newPod, metav1.CreateOptions{})
		if err != nil {
			return errors.Wrap(err, "creating Pod")
		}
	}
	_ = pod
	return nil
}

func (k *KubeHelper) EnsureStatefulSet(ctx context.Context, ssetName string, ssetYAML string) error {
	ssets := k.kube.AppsV1().StatefulSets(k.namespace)
	sset, err := ssets.Get(ctx, ssetName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		var newSset appsv1.StatefulSet
		err := yaml.Unmarshal([]byte(ssetYAML), &newSset)
		if err != nil {
			return errors.Wrap(err, "unmarshaling StatefulSet YAML")
		}
		sset, err = ssets.Create(ctx, &newSset, metav1.CreateOptions{})
		if err != nil {
			return errors.Wrap(err, "creating StatefulSet")
		}
	}
	_ = sset
	return nil
}

func (k *KubeHelper) DeleteStatefulSet(ctx context.Context, ssetName string) error {
	ssets := k.kube.AppsV1().StatefulSets(k.namespace)
	return ssets.Delete(ctx, ssetName, metav1.DeleteOptions{})
}

func (k *KubeHelper) WaitForPodToBeReady(ctx context.Context, podName string, attempts int) error {
	pods := k.kube.CoreV1().Pods(k.namespace)
	for attempts > 0 {
		pod, err := pods.Get(ctx, podName, metav1.GetOptions{})
		if err == nil && pod.Status.Phase == "Running" {
			ready := false
			for _, cond := range pod.Status.Conditions {
				if cond.Type == "Ready" && cond.Status == "True" {
					ready = true
					break
				}
			}
			if ready {
				break
			}
		}
		if err != nil && !k8serrors.IsNotFound(err) {
			return errors.Wrapf(err, "getting pod to wait on (%s)", podName)
		}
		k.logger.Infof("Waiting for pod %s to be ready", podName)
		time.Sleep(time.Second)
		attempts -= 1
	}
	return nil
}

func (k *KubeHelper) PortForward(
	podName string,
	localPort int,
	podPort int,
) (func(), error) {
	stopCh := make(chan struct{}, 1)
	readyCh := make(chan struct{})
	errCh := make(chan error)
	stream := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	go func() {
		err := PortForwardAPod(PortForwardAPodRequest{
			RestConfig: k.config,
			Pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: k.namespace,
				},
			},
			LocalPort: localPort,
			PodPort:   podPort,
			Streams:   stream,
			StopCh:    stopCh,
			ReadyCh:   readyCh,
		})
		if err != nil {
			errCh <- err
		}
	}()

	select {
	case <-readyCh:
		break
	case err := <-errCh:
		return func() {}, err
	}

	return func() { close(stopCh) }, nil
}
