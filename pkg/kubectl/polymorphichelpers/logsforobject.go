/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package polymorphichelpers

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/kubectl/util/podutils"
)

func logsForObject(restClientGetter genericclioptions.RESTClientGetter, object, options runtime.Object, timeout time.Duration, allContainers bool) ([]LogsForObjectResponseWrapper, error) {
	clientConfig, err := restClientGetter.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := corev1client.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	return logsForObjectWithClient(clientset, object, options, timeout, allContainers)
}

// this is split for easy test-ability
func logsForObjectWithClient(clientset corev1client.CoreV1Interface, object, options runtime.Object, timeout time.Duration, allContainers bool) ([]LogsForObjectResponseWrapper, error) {
	opts, ok := options.(*corev1.PodLogOptions)
	if !ok {
		return nil, errors.New("provided options object is not a PodLogOptions")
	}

	switch t := object.(type) {
	case *corev1.PodList:
		ret := []LogsForObjectResponseWrapper{}
		for i := range t.Items {
			currRet, err := logsForObjectWithClient(clientset, &t.Items[i], options, timeout, allContainers)
			if err != nil {
				return nil, err
			}
			ret = append(ret, currRet...)
		}
		return ret, nil

	case *corev1.Pod:
		// if allContainers is true, then we're going to locate all containers and then iterate through them. At that point, "allContainers" is false
		if !allContainers {
			var container *v1.Container
			if opts == nil || len(opts.Container) == 0 {
				// We don't know container name. In this case we expect only one container to be present in the pod (ignoring InitContainers).
				// If there is more than one container we should return an error showing all container names.
				if len(t.Spec.Containers) != 1 {
					containerNames := getContainerNames(t.Spec.Containers)
					initContainerNames := getContainerNames(t.Spec.InitContainers)
					err := fmt.Sprintf("a container name must be specified for pod %s, choose one of: [%s]", t.Name, containerNames)
					if len(initContainerNames) > 0 {
						err += fmt.Sprintf(" or one of the init containers: [%s]", initContainerNames)
					}

					return nil, errors.New(err)
				}
				container = &t.Spec.Containers[0]
			} else {
				container = findContainerByName(t, opts.Container)
				if container == nil {
					return nil, fmt.Errorf("container %s is not valid for pod %s", opts.Container, t.Name)
				}
			}

			request := &logsForObjectRequest{
				Request:   clientset.Pods(t.Namespace).GetLogs(t.Name, opts),
				pod:       t,
				container: container,
			}
			return []LogsForObjectResponseWrapper{request}, nil
		}

		ret := []LogsForObjectResponseWrapper{}
		for _, c := range t.Spec.InitContainers {
			currOpts := opts.DeepCopy()
			currOpts.Container = c.Name
			currRet, err := logsForObjectWithClient(clientset, t, currOpts, timeout, false)
			if err != nil {
				return nil, err
			}
			ret = append(ret, currRet...)
		}
		for _, c := range t.Spec.Containers {
			currOpts := opts.DeepCopy()
			currOpts.Container = c.Name
			currRet, err := logsForObjectWithClient(clientset, t, currOpts, timeout, false)
			if err != nil {
				return nil, err
			}
			ret = append(ret, currRet...)
		}

		return ret, nil
	}

	namespace, selector, err := SelectorsForObject(object)
	if err != nil {
		return nil, fmt.Errorf("cannot get the logs from %T: %v", object, err)
	}

	sortBy := func(pods []*v1.Pod) sort.Interface { return podutils.ByLogging(pods) }
	pod, numPods, err := GetFirstPod(clientset, namespace, selector.String(), timeout, sortBy)
	if err != nil {
		return nil, err
	}
	if numPods > 1 {
		fmt.Fprintf(os.Stderr, "Found %v pods, using pod/%v\n", numPods, pod.Name)
	}

	return logsForObjectWithClient(clientset, pod, options, timeout, allContainers)
}

type logsForObjectRequest struct {
	*rest.Request

	pod       *corev1.Pod
	container *corev1.Container
}

func (l *logsForObjectRequest) SourcePod() *corev1.Pod {
	return l.pod
}

func (l *logsForObjectRequest) SourceContainer() *corev1.Container {
	return l.container
}

// findContainerByName searches for a container by name amongst all containers (including init containers)
// Returns nil, if can't find a matching container.
func findContainerByName(pod *corev1.Pod, name string) *v1.Container {
	for _, c := range pod.Spec.InitContainers {
		if c.Name == name {
			return &c
		}
	}
	for _, c := range pod.Spec.Containers {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

// getContainerNames returns a formatted string containing the container names
func getContainerNames(containers []corev1.Container) string {
	names := []string{}
	for _, c := range containers {
		names = append(names, c.Name)
	}
	return strings.Join(names, " ")
}
