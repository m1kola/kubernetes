/*
Copyright 2017 The Kubernetes Authors.

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
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	fakeexternal "k8s.io/client-go/kubernetes/fake"
	testclient "k8s.io/client-go/testing"
)

var (
	podsResource = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	podsKind     = schema.GroupVersionKind{Version: "v1", Kind: "Pod"}
)

func TestLogsForObject(t *testing.T) {
	tests := []struct {
		name          string
		obj           runtime.Object
		opts          *corev1.PodLogOptions
		allContainers bool
		clientsetPods []runtime.Object
		actions       []testclient.Action

		expectedErr              string
		expectedSourcePods       []*corev1.Pod
		expectedSourceContainers []*corev1.Container
	}{
		{
			name: "pod logs",
			obj:  testPodWithOneContainers(),
			actions: []testclient.Action{
				getLogsAction("test", nil),
			},
			expectedSourcePods: []*corev1.Pod{
				testPodWithOneContainers(),
			},
			expectedSourceContainers: []*corev1.Container{
				&testPodWithOneContainers().Spec.Containers[0],
			},
		},
		{
			name:          "pod logs: all containers",
			obj:           testPodWithTwoContainersAndTwoInitContainers(),
			opts:          &corev1.PodLogOptions{},
			allContainers: true,
			actions: []testclient.Action{
				getLogsAction("test", &corev1.PodLogOptions{Container: "foo-2-and-2-initc1"}),
				getLogsAction("test", &corev1.PodLogOptions{Container: "foo-2-and-2-initc2"}),
				getLogsAction("test", &corev1.PodLogOptions{Container: "foo-2-and-2-c1"}),
				getLogsAction("test", &corev1.PodLogOptions{Container: "foo-2-and-2-c2"}),
			},
			expectedSourcePods: []*corev1.Pod{
				testPodWithTwoContainersAndTwoInitContainers(),
				testPodWithTwoContainersAndTwoInitContainers(),
				testPodWithTwoContainersAndTwoInitContainers(),
				testPodWithTwoContainersAndTwoInitContainers(),
			},
			expectedSourceContainers: []*corev1.Container{
				&testPodWithTwoContainersAndTwoInitContainers().Spec.InitContainers[0],
				&testPodWithTwoContainersAndTwoInitContainers().Spec.InitContainers[1],
				&testPodWithTwoContainersAndTwoInitContainers().Spec.Containers[0],
				&testPodWithTwoContainersAndTwoInitContainers().Spec.Containers[1],
			},
		},
		{
			name:        "pod logs: error - must provide container name",
			obj:         testPodWithTwoContainers(),
			expectedErr: "a container name must be specified for pod foo-two-containers, choose one of: [foo-2-c1 foo-2-c2]",
		},
		{
			name: "pods list logs",
			obj: &corev1.PodList{
				Items: []corev1.Pod{*testPodWithOneContainers()},
			},
			actions: []testclient.Action{
				getLogsAction("test", nil),
			},
			expectedSourcePods: []*corev1.Pod{
				testPodWithOneContainers(),
			},
			expectedSourceContainers: []*corev1.Container{
				&testPodWithOneContainers().Spec.Containers[0],
			},
		},
		{
			name: "pods list logs: all containers",
			obj: &corev1.PodList{
				Items: []corev1.Pod{*testPodWithTwoContainersAndTwoInitContainers()},
			},
			opts:          &corev1.PodLogOptions{},
			allContainers: true,
			actions: []testclient.Action{
				getLogsAction("test", &corev1.PodLogOptions{Container: "foo-2-and-2-initc1"}),
				getLogsAction("test", &corev1.PodLogOptions{Container: "foo-2-and-2-initc2"}),
				getLogsAction("test", &corev1.PodLogOptions{Container: "foo-2-and-2-c1"}),
				getLogsAction("test", &corev1.PodLogOptions{Container: "foo-2-and-2-c2"}),
			},
			expectedSourcePods: []*corev1.Pod{
				testPodWithTwoContainersAndTwoInitContainers(),
				testPodWithTwoContainersAndTwoInitContainers(),
				testPodWithTwoContainersAndTwoInitContainers(),
				testPodWithTwoContainersAndTwoInitContainers(),
			},
			expectedSourceContainers: []*corev1.Container{
				&testPodWithTwoContainersAndTwoInitContainers().Spec.InitContainers[0],
				&testPodWithTwoContainersAndTwoInitContainers().Spec.InitContainers[1],
				&testPodWithTwoContainersAndTwoInitContainers().Spec.Containers[0],
				&testPodWithTwoContainersAndTwoInitContainers().Spec.Containers[1],
			},
		},
		{
			name: "pods list logs: error - must provide container name",
			obj: &corev1.PodList{
				Items: []corev1.Pod{*testPodWithTwoContainersAndTwoInitContainers()},
			},
			expectedErr: "a container name must be specified for pod foo-two-containers-and-two-init-containers, choose one of: [foo-2-and-2-c1 foo-2-and-2-c2] or one of the init containers: [foo-2-and-2-initc1 foo-2-and-2-initc2]",
		},
		{
			name: "replication controller logs",
			obj: &corev1.ReplicationController{
				ObjectMeta: metav1.ObjectMeta{Name: "hello", Namespace: "test"},
				Spec: corev1.ReplicationControllerSpec{
					Selector: map[string]string{"foo": "bar"},
				},
			},
			clientsetPods: []runtime.Object{testPodWithOneContainers()},
			actions: []testclient.Action{
				testclient.NewListAction(podsResource, podsKind, "test", metav1.ListOptions{LabelSelector: "foo=bar"}),
				getLogsAction("test", nil),
			},
			expectedSourcePods: []*corev1.Pod{
				testPodWithOneContainers(),
			},
			expectedSourceContainers: []*corev1.Container{
				&testPodWithOneContainers().Spec.Containers[0],
			},
		},
		{
			name: "replica set logs",
			obj: &extensionsv1beta1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{Name: "hello", Namespace: "test"},
				Spec: extensionsv1beta1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
			},
			clientsetPods: []runtime.Object{testPodWithOneContainers()},
			actions: []testclient.Action{
				testclient.NewListAction(podsResource, podsKind, "test", metav1.ListOptions{LabelSelector: "foo=bar"}),
				getLogsAction("test", nil),
			},
			expectedSourcePods: []*corev1.Pod{
				testPodWithOneContainers(),
			},
			expectedSourceContainers: []*corev1.Container{
				&testPodWithOneContainers().Spec.Containers[0],
			},
		},
		{
			name: "deployment logs",
			obj: &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "hello", Namespace: "test"},
				Spec: extensionsv1beta1.DeploymentSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
			},
			clientsetPods: []runtime.Object{testPodWithOneContainers()},
			actions: []testclient.Action{
				testclient.NewListAction(podsResource, podsKind, "test", metav1.ListOptions{LabelSelector: "foo=bar"}),
				getLogsAction("test", nil),
			},
			expectedSourcePods: []*corev1.Pod{
				testPodWithOneContainers(),
			},
			expectedSourceContainers: []*corev1.Container{
				&testPodWithOneContainers().Spec.Containers[0],
			},
		},
		{
			name: "job logs",
			obj: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "hello", Namespace: "test"},
				Spec: batchv1.JobSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
			},
			clientsetPods: []runtime.Object{testPodWithOneContainers()},
			actions: []testclient.Action{
				testclient.NewListAction(podsResource, podsKind, "test", metav1.ListOptions{LabelSelector: "foo=bar"}),
				getLogsAction("test", nil),
			},
			expectedSourcePods: []*corev1.Pod{
				testPodWithOneContainers(),
			},
			expectedSourceContainers: []*corev1.Container{
				&testPodWithOneContainers().Spec.Containers[0],
			},
		},
		{
			name: "stateful set logs",
			obj: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: "hello", Namespace: "test"},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
			},
			clientsetPods: []runtime.Object{testPodWithOneContainers()},
			actions: []testclient.Action{
				testclient.NewListAction(podsResource, podsKind, "test", metav1.ListOptions{LabelSelector: "foo=bar"}),
				getLogsAction("test", nil),
			},
			expectedSourcePods: []*corev1.Pod{
				testPodWithOneContainers(),
			},
			expectedSourceContainers: []*corev1.Container{
				&testPodWithOneContainers().Spec.Containers[0],
			},
		},
	}

	for _, test := range tests {
		fakeClientset := fakeexternal.NewSimpleClientset(test.clientsetPods...)
		responses, err := logsForObjectWithClient(fakeClientset.CoreV1(), test.obj, test.opts, 20*time.Second, test.allContainers)
		if test.expectedErr == "" && err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}

		if err != nil && test.expectedErr != err.Error() {
			t.Errorf("%s: expected error: %v, got: %v", test.name, test.expectedErr, err)
			continue
		}

		if len(test.expectedSourcePods) != len(responses) {
			t.Errorf(
				"%s: the number of expected source pods doesn't match the number of responses: %v, got: %v",
				test.name,
				len(test.expectedSourcePods),
				len(responses),
			)
			continue
		}

		if len(test.expectedSourceContainers) != len(responses) {
			t.Errorf(
				"%s: the number of expected source containers doesn't match the number of responses: %v, got: %v",
				test.name,
				len(test.expectedSourceContainers),
				len(responses),
			)
			continue
		}

		for i := range test.expectedSourcePods {
			got := responses[i].SourcePod()
			want := test.expectedSourcePods[i]
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: unexpected source pod: %s", test.name, diff.ObjectDiff(got, want))
			}
		}

		for i := range test.expectedSourcePods {
			got := responses[i].SourceContainer()
			want := test.expectedSourceContainers[i]
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: unexpected source container: %s", test.name, diff.ObjectDiff(got, want))
			}
		}

		for i := range test.actions {
			if len(fakeClientset.Actions()) < i {
				t.Errorf("%s: action %d does not exists in actual actions: %#v",
					test.name, i, fakeClientset.Actions())
				continue
			}
			got := fakeClientset.Actions()[i]
			want := test.actions[i]
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: unexpected action: %s", test.name, diff.ObjectDiff(got, want))
			}
		}
	}
}

func testPodWithOneContainers() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "test",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			DNSPolicy:     corev1.DNSClusterFirst,
			Containers: []corev1.Container{
				{Name: "c1"},
			},
		},
	}
}

func testPodWithTwoContainers() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-two-containers",
			Namespace: "test",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			DNSPolicy:     corev1.DNSClusterFirst,
			Containers: []corev1.Container{
				{Name: "foo-2-c1"},
				{Name: "foo-2-c2"},
			},
		},
	}
}

func testPodWithTwoContainersAndTwoInitContainers() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-two-containers-and-two-init-containers",
			Namespace: "test",
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "foo-2-and-2-initc1"},
				{Name: "foo-2-and-2-initc2"},
			},
			Containers: []corev1.Container{
				{Name: "foo-2-and-2-c1"},
				{Name: "foo-2-and-2-c2"},
			},
		},
	}
}

func getLogsAction(namespace string, opts *corev1.PodLogOptions) testclient.Action {
	action := testclient.GenericActionImpl{}
	action.Verb = "get"
	action.Namespace = namespace
	action.Resource = podsResource
	action.Subresource = "log"
	action.Value = opts
	return action
}
